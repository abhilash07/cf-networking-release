package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"lib/filelock"
	"lib/rules"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"code.cloudfoundry.org/garden"

	"github.com/coreos/go-iptables/iptables"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf-experimental/gomegamatchers"
)

type fakePluginLogData struct {
	Args  []string
	Env   map[string]string
	Stdin string
}

func expectedStdin_CNI_ADD(index int) string {
	return fmt.Sprintf(`
	{
		"cniVersion": "0.1.0",
		"name": "some-net-%d",
		"type": "plugin-%d",
		"metadata": {
				"some-key": "some-value",
				"policy_group_id": "some-group-id"
		}
	}`, index, index)
}
func expectedStdin_CNI_DEL(index int) string {
	return fmt.Sprintf(`
	{
		"cniVersion": "0.1.0",
		"name": "some-net-%d",
		"type": "plugin-%d"
	}`, index, index)
}

func writeConfig(index int, outDir string) error {
	config := fmt.Sprintf(`
	{
		"cniVersion": "0.1.0",
		"name": "some-net-%d",
		"type": "plugin-%d"
	}`, index, index)
	outpath := filepath.Join(outDir, fmt.Sprintf("%d-plugin-%d.conf", 10*index, index))
	return ioutil.WriteFile(outpath, []byte(config), 0600)
}

func sameFile(path1, path2 string) bool {
	file1, err := os.Stat(path1)
	Expect(err).NotTo(HaveOccurred())

	file2, err := os.Stat(path2)
	Expect(err).NotTo(HaveOccurred())
	return os.SameFile(file1, file2)
}

const DEFAULT_TIMEOUT = "10s"
const GlobalIPTablesLockFile = "/tmp/netman/iptables.lock"

func buildStdin(inputs interface{}) io.Reader {
	jsonBytes, err := json.Marshal(inputs)
	Expect(err).NotTo(HaveOccurred())
	return bytes.NewReader(jsonBytes)
}

var _ = Describe("Garden External Networker", func() {
	var (
		cniConfigDir           string
		fakePid                int
		fakeLogDir             string
		expectedNetNSPath      string
		bindMountRoot          string
		stateFilePath          string
		containerHandle        string
		netoutChainName        string
		inputChainName         string
		netoutLoggingChainName string
		netinChainName         string
		fakeProcess            *os.Process
		fakeConfigFilePath     string
		upCommand, downCommand *exec.Cmd
	)

	BeforeEach(func() {
		var err error
		cniConfigDir, err = ioutil.TempDir("", "cni-config-")
		Expect(err).NotTo(HaveOccurred())

		fakeLogDir, err = ioutil.TempDir("", "fake-logs-")
		Expect(err).NotTo(HaveOccurred())

		containerHandle = fmt.Sprintf("container-%04x-%x", GinkgoParallelNode(), rand.Int63())
		netoutChainName = fmt.Sprintf("netout--%s", containerHandle)[:28]
		inputChainName = fmt.Sprintf("input--%s", containerHandle)[:28]
		netinChainName = fmt.Sprintf("netin--%s", containerHandle)[:28]
		netoutLoggingChainName = fmt.Sprintf("%s--log", netoutChainName[:23])

		sleepCmd := exec.Command("/bin/sleep", "1000")
		Expect(sleepCmd.Start()).To(Succeed())
		fakeProcess = sleepCmd.Process

		fakePid = fakeProcess.Pid

		bindMountRoot, err = ioutil.TempDir("", "bind-mount-root")
		Expect(err).NotTo(HaveOccurred())

		expectedNetNSPath = fmt.Sprintf("%s/%s", bindMountRoot, containerHandle)

		stateFile, err := ioutil.TempFile("", "external-networker-state.json")
		Expect(err).NotTo(HaveOccurred())
		Expect(stateFile.Close()).To(Succeed())
		stateFilePath = stateFile.Name()

		Expect(writeConfig(0, cniConfigDir)).To(Succeed())
		Expect(writeConfig(1, cniConfigDir)).To(Succeed())
		Expect(writeConfig(2, cniConfigDir)).To(Succeed())

		configFile, err := ioutil.TempFile("", "adapter-config-")
		Expect(err).NotTo(HaveOccurred())
		fakeConfigFilePath = configFile.Name()
		config := map[string]interface{}{
			"cni_plugin_dir":     paths.CniPluginDir,
			"cni_config_dir":     cniConfigDir,
			"bind_mount_dir":     bindMountRoot,
			"overlay_network":    "10.255.0.0/16",
			"state_file":         stateFilePath,
			"start_port":         60000,
			"total_ports":        56,
			"iptables_lock_file": GlobalIPTablesLockFile,
			"instance_address":   "1.2.3.4",
		}
		configBytes, err := json.Marshal(config)
		Expect(err).NotTo(HaveOccurred())
		_, err = configFile.Write(configBytes)
		Expect(err).NotTo(HaveOccurred())
		Expect(configFile.Close()).To(Succeed())

		upCommand = exec.Command(paths.PathToAdapter)
		upCommand.Env = append(os.Environ(), "FAKE_LOG_DIR="+fakeLogDir)
		upCommand.Stdin = buildStdin(map[string]interface{}{
			"pid": fakePid,
			"properties": map[string]string{
				"some-key":        "some-value",
				"policy_group_id": "some-group-id",
			},
			"netin": []map[string]int{
				{
					"host_port":      12345,
					"container_port": 7000,
				},
			},
			"netout_rules": []map[string]interface{}{
				{
					"protocol": 1,
					"networks": []map[string]string{
						{
							"start": "8.8.8.8",
							"end":   "9.9.9.9",
						},
					},
					"ports": []map[string]int{
						{
							"start": 53,
							"end":   54,
						},
					},
					"log": true,
				},
			},
		},
		)
		upCommand.Args = []string{
			paths.PathToAdapter,
			"--configFile", fakeConfigFilePath,
			"--action", "up",
			"--handle", containerHandle,
		}

		downCommand = exec.Command(paths.PathToAdapter)
		downCommand.Env = append(os.Environ(), "FAKE_LOG_DIR="+fakeLogDir)
		downCommand.Stdin = strings.NewReader(`{}`)
		downCommand.Args = []string{
			paths.PathToAdapter,
			"--action", "down",
			"--handle", containerHandle,
			"--configFile", fakeConfigFilePath,
		}
	})

	AfterEach(func() {
		Expect(os.Remove(fakeConfigFilePath)).To(Succeed())
		Expect(os.RemoveAll(cniConfigDir)).To(Succeed())
		Expect(os.RemoveAll(fakeLogDir)).To(Succeed())
		Expect(fakeProcess.Kill()).To(Succeed())

		ipt, err := iptables.New()
		Expect(err).NotTo(HaveOccurred())
		iptLocker := &rules.IPTablesLocker{
			FileLocker: &filelock.Locker{Path: "/var/run/netman-iptables.lock"},
			Mutex:      &sync.Mutex{},
		}
		restorer := &rules.Restorer{}
		lockedIPTables := &rules.LockedIPTables{
			IPTables: ipt,
			Locker:   iptLocker,
			Restorer: restorer,
		}
		Expect(lockedIPTables.ClearChain("filter", netoutLoggingChainName)).To(Succeed())
		Expect(lockedIPTables.ClearChain("filter", netoutChainName)).To(Succeed())
		Expect(lockedIPTables.ClearChain("filter", inputChainName)).To(Succeed())
		Expect(lockedIPTables.ClearChain("filter", "FORWARD")).To(Succeed())
		Expect(lockedIPTables.ClearChain("filter", "INPUT")).To(Succeed())
		Expect(lockedIPTables.DeleteChain("filter", netoutLoggingChainName)).To(Succeed())
		Expect(lockedIPTables.DeleteChain("filter", netoutChainName)).To(Succeed())
		Expect(lockedIPTables.DeleteChain("filter", inputChainName)).To(Succeed())
		Expect(lockedIPTables.ClearChain("nat", netinChainName)).To(Succeed())
		Expect(lockedIPTables.ClearChain("nat", "PREROUTING")).To(Succeed())
		Expect(lockedIPTables.DeleteChain("nat", netinChainName)).To(Succeed())

		// assert that the test returns iptables to a pristine state when finished
		Expect(AllIPTablesRules("filter")).To(ConsistOf("-P INPUT ACCEPT", "-P FORWARD ACCEPT", "-P OUTPUT ACCEPT"))
	})

	It("should call CNI ADD and DEL", func() {
		By("calling up")
		upSession := runAndWait(upCommand)
		Expect(upSession.Out.Contents()).To(MatchJSON(`{ "properties": {
			"garden.network.container-ip": "169.254.1.2",
			"garden.network.host-ip": "255.255.255.255",
			"garden.network.mapped-ports": "[{\"HostPort\":12345,\"ContainerPort\":7000}]"
			}
		}`))

		By("checking that it logs basic info on stderr")
		Expect(upSession.Err).To(gbytes.Say("container-networking.garden-external-networker.*action.*up"))

		By("checking that every CNI plugin in the plugin directory got called with ADD")
		for i := 0; i < 3; i++ {
			logFileContents, err := ioutil.ReadFile(filepath.Join(fakeLogDir, fmt.Sprintf("plugin-%d.log", i)))
			Expect(err).NotTo(HaveOccurred())
			var pluginCallInfo fakePluginLogData
			Expect(json.Unmarshal(logFileContents, &pluginCallInfo)).To(Succeed())

			Expect(pluginCallInfo.Stdin).To(MatchJSON(expectedStdin_CNI_ADD(i)))
			Expect(pluginCallInfo.Env).To(HaveKeyWithValue("CNI_COMMAND", "ADD"))
			Expect(pluginCallInfo.Env).To(HaveKeyWithValue("CNI_CONTAINERID", containerHandle))
			Expect(pluginCallInfo.Env).To(HaveKeyWithValue("CNI_IFNAME", fmt.Sprintf("eth%d", i)))
			Expect(pluginCallInfo.Env).To(HaveKeyWithValue("CNI_PATH", paths.CniPluginDir))
			Expect(pluginCallInfo.Env).To(HaveKeyWithValue("CNI_NETNS", expectedNetNSPath))
			Expect(pluginCallInfo.Env).To(HaveKeyWithValue("CNI_ARGS", ""))
		}

		By("checking that the fake process's network namespace has been bind-mounted into the filesystem")
		Expect(sameFile(expectedNetNSPath, fmt.Sprintf("/proc/%d/ns/net", fakePid))).To(BeTrue())

		By("calling down")
		runAndWait(downCommand)

		By("checking that every CNI plugin in the plugin directory got called with DEL")
		for i := 0; i < 3; i++ {
			logFileContents, err := ioutil.ReadFile(filepath.Join(fakeLogDir, fmt.Sprintf("plugin-%d.log", i)))
			Expect(err).NotTo(HaveOccurred())
			var pluginCallInfo fakePluginLogData
			Expect(json.Unmarshal(logFileContents, &pluginCallInfo)).To(Succeed())

			Expect(pluginCallInfo.Stdin).To(MatchJSON(expectedStdin_CNI_DEL(i)))
			Expect(pluginCallInfo.Env).To(HaveKeyWithValue("CNI_COMMAND", "DEL"))
			Expect(pluginCallInfo.Env).To(HaveKeyWithValue("CNI_CONTAINERID", containerHandle))
			Expect(pluginCallInfo.Env).To(HaveKeyWithValue("CNI_IFNAME", fmt.Sprintf("eth%d", i)))
			Expect(pluginCallInfo.Env).To(HaveKeyWithValue("CNI_PATH", paths.CniPluginDir))
			Expect(pluginCallInfo.Env).To(HaveKeyWithValue("CNI_NETNS", expectedNetNSPath))
			Expect(pluginCallInfo.Env).To(HaveKeyWithValue("CNI_ARGS", ""))
		}

		By("checking that the bind-mounted namespace has been removed")
		Expect(expectedNetNSPath).NotTo(BeAnExistingFile())

		By("seeing that is succeeds when calling down again")
		downCommand2 := exec.Command(paths.PathToAdapter)
		downCommand2.Env = append(os.Environ(), "FAKE_LOG_DIR="+fakeLogDir)
		downCommand2.Stdin = strings.NewReader(`{}`)
		downCommand2.Args = []string{
			paths.PathToAdapter,
			"--action", "down",
			"--handle", containerHandle,
			"--configFile", fakeConfigFilePath,
		}
		runAndWait(downCommand2)
	})

	Describe("BulkNetOut lifecycle", func() {
		var buildBulkNetOutCommand = func(containerIP string, rules []garden.NetOutRule) *exec.Cmd {
			bulkNetOutCommand := exec.Command(paths.PathToAdapter)
			bulkNetOutCommand.Env = append(os.Environ(), "FAKE_LOG_DIR="+fakeLogDir)
			bulkNetOutCommand.Stdin = buildStdin(map[string]interface{}{
				"container_ip": containerIP,
				"netout_rules": rules,
			})
			bulkNetOutCommand.Args = []string{
				paths.PathToAdapter,
				"--action", "bulk-net-out",
				"--handle", containerHandle,
				"--configFile", fakeConfigFilePath,
			}
			return bulkNetOutCommand
		}

		var someRules []garden.NetOutRule
		BeforeEach(func() {
			for i := 0; i < 5; i++ {
				rule := garden.NetOutRule{
					Protocol: garden.ProtocolTCP,
					Networks: []garden.IPRange{
						{Start: net.ParseIP(fmt.Sprintf("1.1.1.%d", i+1)), End: net.ParseIP("2.2.2.2")},
					},
					Ports: []garden.PortRange{{Start: 9000, End: 9999}},
				}
				if i == 0 {
					rule.Log = true
				}
				someRules = append(someRules, rule)
			}
		})

		It("it writes NetOut rules in bulk", func() {
			By("calling up")
			runAndWait(upCommand)

			By("checking that the default forwarding rules are created for that container")
			Expect(AllIPTablesRules("filter")).To(gomegamatchers.ContainSequence([]string{
				`-A ` + netoutChainName + ` -s 169.254.1.2/32 ! -d 10.255.0.0/16 -m state --state RELATED,ESTABLISHED -j RETURN`,
				`-A ` + netoutChainName + ` -s 169.254.1.2/32 ! -d 10.255.0.0/16 -j REJECT --reject-with icmp-port-unreachable`,
			}))

			By("checking that the default input rules are created for that container")
			Expect(AllIPTablesRules("filter")).To(gomegamatchers.ContainSequence([]string{
				`-A ` + inputChainName + ` -s 169.254.1.2/32 -m state --state RELATED,ESTABLISHED -j RETURN`,
				`-A ` + inputChainName + ` -s 169.254.1.2/32 -j REJECT --reject-with icmp-port-unreachable`,
			}))

			By("checking that the rules provided in the up command are written")
			Expect(AllIPTablesRules("filter")).To(ContainElement(`-A ` + netoutChainName + ` -s 169.254.1.2/32 -p tcp -m iprange --dst-range 8.8.8.8-9.9.9.9 -m tcp --dport 53:54 -g ` + netoutLoggingChainName))

			By("calling bulk netout")
			bulkNetOutCommand := buildBulkNetOutCommand("169.254.1.2", someRules)
			runAndWait(bulkNetOutCommand)

			By("checking that the filter rule was installed and that logging can be enabled")
			Expect(AllIPTablesRules("filter")).To(ContainElement(`-A ` + netoutChainName + ` -s 169.254.1.2/32 -p tcp -m iprange --dst-range 1.1.1.1-2.2.2.2 -m tcp --dport 9000:9999 -g ` + netoutLoggingChainName))
			Expect(AllIPTablesRules("filter")).To(ContainElement(`-A ` + netoutChainName + ` -s 169.254.1.2/32 -p tcp -m iprange --dst-range 1.1.1.2-2.2.2.2 -m tcp --dport 9000:9999 -j RETURN`))
			Expect(AllIPTablesRules("filter")).To(ContainElement(`-A ` + netoutChainName + ` -s 169.254.1.2/32 -p tcp -m iprange --dst-range 1.1.1.3-2.2.2.2 -m tcp --dport 9000:9999 -j RETURN`))
			Expect(AllIPTablesRules("filter")).To(ContainElement(`-A ` + netoutChainName + ` -s 169.254.1.2/32 -p tcp -m iprange --dst-range 1.1.1.4-2.2.2.2 -m tcp --dport 9000:9999 -j RETURN`))
			Expect(AllIPTablesRules("filter")).To(ContainElement(`-A ` + netoutChainName + ` -s 169.254.1.2/32 -p tcp -m iprange --dst-range 1.1.1.5-2.2.2.2 -m tcp --dport 9000:9999 -j RETURN`))

			By("checking that it writes the logging rules")
			Expect(AllIPTablesRules("filter")).To(ContainElement(`-A ` + netoutLoggingChainName + ` -p tcp -m conntrack --ctstate INVALID,NEW,UNTRACKED -j LOG --log-prefix OK_` + containerHandle[:26]))

			By("calling down")
			runAndWait(downCommand)

			By("checking that there are no more netout rules for this container")
			Expect(AllIPTablesRules("filter")).NotTo(ContainElement(ContainSubstring(inputChainName)))
			Expect(AllIPTablesRules("filter")).NotTo(ContainElement(ContainSubstring(netoutChainName)))
			Expect(AllIPTablesRules("filter")).NotTo(ContainElement(ContainSubstring(netoutLoggingChainName)))
		})

	})

	Describe("NetOut rule lifecycle", func() {
		var buildNetOutCommand = func(containerIP string, rule garden.NetOutRule) *exec.Cmd {
			netOutCommand := exec.Command(paths.PathToAdapter)
			netOutCommand.Env = append(os.Environ(), "FAKE_LOG_DIR="+fakeLogDir)
			netOutCommand.Stdin = buildStdin(map[string]interface{}{
				"container_ip": containerIP,
				"netout_rule":  rule,
			})
			netOutCommand.Args = []string{
				paths.PathToAdapter,
				"--action", "net-out",
				"--handle", containerHandle,
				"--configFile", fakeConfigFilePath,
			}
			return netOutCommand
		}
		var someRule garden.NetOutRule
		BeforeEach(func() {
			someRule = garden.NetOutRule{
				Protocol: garden.ProtocolTCP,
				Networks: []garden.IPRange{
					{Start: net.ParseIP("1.1.1.1"), End: net.ParseIP("2.2.2.2")},
				},
				Ports: []garden.PortRange{{Start: 9000, End: 9999}},
			}
		})

		Context("when asg iptables logging is enabled", func() {
			BeforeEach(func() {
				configFile, err := ioutil.TempFile("", "adapter-config-")
				Expect(err).NotTo(HaveOccurred())
				fakeConfigFilePath = configFile.Name()
				config := map[string]interface{}{
					"cni_plugin_dir":       paths.CniPluginDir,
					"cni_config_dir":       cniConfigDir,
					"bind_mount_dir":       bindMountRoot,
					"overlay_network":      "10.255.0.0/16",
					"state_file":           stateFilePath,
					"start_port":           60000,
					"total_ports":          56,
					"iptables_lock_file":   GlobalIPTablesLockFile,
					"instance_address":     "1.2.3.4",
					"iptables_asg_logging": true,
				}
				configBytes, err := json.Marshal(config)
				Expect(err).NotTo(HaveOccurred())
				_, err = configFile.Write(configBytes)
				Expect(err).NotTo(HaveOccurred())
				Expect(configFile.Close()).To(Succeed())

				upCommand.Args = []string{
					paths.PathToAdapter,
					"--configFile", fakeConfigFilePath,
					"--action", "up",
					"--handle", containerHandle,
				}
			})

			It("writes a default deny logging rule", func() {
				By("calling up")
				upSession := runAndWait(upCommand)
				Eventually(upSession.Exited).Should(BeClosed())

				By("checking that default deny logging rule is created")
				Expect(AllIPTablesRules("filter")).To(gomegamatchers.ContainSequence([]string{
					`-A ` + netoutChainName + ` -s 169.254.1.2/32 ! -d 10.255.0.0/16 -j LOG --log-prefix DENY_` + containerHandle[:24],
					`-A ` + netoutChainName + ` -s 169.254.1.2/32 ! -d 10.255.0.0/16 -j REJECT --reject-with icmp-port-unreachable`,
				}))

				By("calling netout")
				netOutCommand := buildNetOutCommand("169.254.1.2", someRule)
				runAndWait(netOutCommand)

				By("checking that the accept logging rule is created")
				Expect(AllIPTablesRules("filter")).To(ContainElement(`-A ` + netoutChainName + ` -s 169.254.1.2/32 -p tcp -m iprange --dst-range 1.1.1.1-2.2.2.2 -m tcp --dport 9000:9999 -g ` + netoutLoggingChainName))

			})

		})

		It("writes NetOut rules", func() {
			By("calling up")
			upSession := runAndWait(upCommand)
			Expect(upSession.Out.Contents()).To(MatchJSON(`{ "properties": {
				"garden.network.container-ip": "169.254.1.2",
				"garden.network.host-ip": "255.255.255.255",
				"garden.network.mapped-ports": "[{\"HostPort\":12345,\"ContainerPort\":7000}]"
				}
			}`))

			By("checking that the default rules are created for that container")
			Expect(AllIPTablesRules("filter")).To(gomegamatchers.ContainSequence([]string{
				`-A ` + netoutChainName + ` -s 169.254.1.2/32 ! -d 10.255.0.0/16 -m state --state RELATED,ESTABLISHED -j RETURN`,
				`-A ` + netoutChainName + ` -s 169.254.1.2/32 ! -d 10.255.0.0/16 -j REJECT --reject-with icmp-port-unreachable`,
			}))

			By("calling netout")
			netOutCommand := buildNetOutCommand("169.254.1.2", someRule)
			runAndWait(netOutCommand)

			By("checking that the filter rule was installed")
			Expect(AllIPTablesRules("filter")).To(ContainElement(`-A ` + netoutChainName + ` -s 169.254.1.2/32 -p tcp -m iprange --dst-range 1.1.1.1-2.2.2.2 -m tcp --dport 9000:9999 -j RETURN`))

			By("calling netout again but without ports or protocols")
			someRule.Ports = nil
			someRule.Protocol = 0
			someRule.Networks = []garden.IPRange{
				{Start: net.ParseIP("3.3.3.3"), End: net.ParseIP("4.4.4.4")},
			}
			netOutCommand = buildNetOutCommand("169.254.1.2", someRule)
			runAndWait(netOutCommand)

			By("checking that both filter rules were installed")
			Expect(AllIPTablesRules("filter")).To(ContainElement(`-A ` + netoutChainName + ` -s 169.254.1.2/32 -p tcp -m iprange --dst-range 1.1.1.1-2.2.2.2 -m tcp --dport 9000:9999 -j RETURN`))
			Expect(AllIPTablesRules("filter")).To(ContainElement(`-A ` + netoutChainName + ` -s 169.254.1.2/32 -m iprange --dst-range 3.3.3.3-4.4.4.4 -j RETURN`))

			By("calling down")
			runAndWait(downCommand)

			By("checking that there are no more netout rules for this container")
			Expect(AllIPTablesRules("filter")).NotTo(ContainElement(ContainSubstring(netoutChainName)))
		})
	})

	Describe("NetIn rule lifecycle", func() {
		var netInCommand *exec.Cmd

		BeforeEach(func() {
			netInCommand = exec.Command(paths.PathToAdapter)
			netInCommand.Env = append(os.Environ(), "FAKE_LOG_DIR="+fakeLogDir)
			netInCommand.Stdin = buildStdin(map[string]interface{}{
				"HostIP":        "1.2.3.4",
				"HostPort":      0,
				"ContainerIp":   "169.254.1.2",
				"ContainerPort": 8080,
			})
			netInCommand.Args = []string{
				paths.PathToAdapter,
				"--action", "net-in",
				"--handle", containerHandle,
				"--configFile", fakeConfigFilePath,
			}
		})

		It("writes iptables rules for NetIn", func() {
			By("calling up")
			upSession := runAndWait(upCommand)
			Expect(upSession.Out.Contents()).To(MatchJSON(`{ "properties": {
				"garden.network.container-ip": "169.254.1.2",
				"garden.network.host-ip": "255.255.255.255",
				"garden.network.mapped-ports": "[{\"HostPort\":12345,\"ContainerPort\":7000}]"
				}
			}`))

			By("checking that a netin chain was created for the container")
			Expect(AllIPTablesRules("nat")).To(ContainElement(`-N ` + netinChainName))
			Expect(AllIPTablesRules("nat")).To(ContainElement(`-A PREROUTING -j ` + netinChainName))

			By("checking that the rules passed in on the up call are written")
			Expect(AllIPTablesRules("nat")).To(ContainElement(`-A ` + netinChainName + ` -d 1.2.3.4/32 -p tcp -m tcp --dport 12345 -j DNAT --to-destination 169.254.1.2:7000`))

			By("calling netin")
			netInSession := runAndWait(netInCommand)

			By("checking the return host and container port")
			var result struct {
				HostPort      int `json:"host_port"`
				ContainerPort int `json:"container_port"`
			}
			Expect(json.Unmarshal(netInSession.Out.Contents(), &result)).To(Succeed())
			Expect(result.HostPort).To(Equal(60000))
			Expect(result.ContainerPort).To(Equal(8080))

			By("checking that a port forwarding rule was added to the netin chain")
			Expect(AllIPTablesRules("nat")).To(ContainElement(`-A ` + netinChainName + ` -d 1.2.3.4/32 -p tcp -m tcp --dport 60000 -j DNAT --to-destination 169.254.1.2:8080`))

			By("seeing that the allocated port is stored to the state file on disk")
			stateFileBytes, err := ioutil.ReadFile(stateFilePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(stateFileBytes).To(ContainSubstring(fmt.Sprintf("%d", result.HostPort)))

			By("calling down")
			runAndWait(downCommand)

			By("checking that there are no more netin rules for this container")
			Expect(AllIPTablesRules("nat")).NotTo(ContainElement(ContainSubstring(netinChainName)))

			By("seeing that the port is released from the state file on disk")
			stateFileBytes, err = ioutil.ReadFile(stateFilePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(stateFileBytes).NotTo(ContainSubstring(fmt.Sprintf("%d", result.HostPort)))
		})
	})
})

func AllIPTablesRules(tableName string) []string {
	iptablesSession, err := gexec.Start(exec.Command("iptables", "-w", "-S", "-t", tableName), GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(iptablesSession).Should(gexec.Exit(0))
	return strings.Split(strings.TrimSpace(string(iptablesSession.Out.Contents())), "\n")
}

func runAndWait(cmd *exec.Cmd) *gexec.Session {
	session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	Eventually(session, DEFAULT_TIMEOUT).Should(gexec.Exit(0))
	return session
}
