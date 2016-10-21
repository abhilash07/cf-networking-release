// This file was generated by counterfeiter
package fakes

import "sync"

type OrgDeleterCli struct {
	DeleteOrgStub        func(name string) error
	deleteOrgMutex       sync.RWMutex
	deleteOrgArgsForCall []struct {
		name string
	}
	deleteOrgReturns struct {
		result1 error
	}
	DeleteQuotaStub        func(name string) error
	deleteQuotaMutex       sync.RWMutex
	deleteQuotaArgsForCall []struct {
		name string
	}
	deleteQuotaReturns struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *OrgDeleterCli) DeleteOrg(name string) error {
	fake.deleteOrgMutex.Lock()
	fake.deleteOrgArgsForCall = append(fake.deleteOrgArgsForCall, struct {
		name string
	}{name})
	fake.recordInvocation("DeleteOrg", []interface{}{name})
	fake.deleteOrgMutex.Unlock()
	if fake.DeleteOrgStub != nil {
		return fake.DeleteOrgStub(name)
	} else {
		return fake.deleteOrgReturns.result1
	}
}

func (fake *OrgDeleterCli) DeleteOrgCallCount() int {
	fake.deleteOrgMutex.RLock()
	defer fake.deleteOrgMutex.RUnlock()
	return len(fake.deleteOrgArgsForCall)
}

func (fake *OrgDeleterCli) DeleteOrgArgsForCall(i int) string {
	fake.deleteOrgMutex.RLock()
	defer fake.deleteOrgMutex.RUnlock()
	return fake.deleteOrgArgsForCall[i].name
}

func (fake *OrgDeleterCli) DeleteOrgReturns(result1 error) {
	fake.DeleteOrgStub = nil
	fake.deleteOrgReturns = struct {
		result1 error
	}{result1}
}

func (fake *OrgDeleterCli) DeleteQuota(name string) error {
	fake.deleteQuotaMutex.Lock()
	fake.deleteQuotaArgsForCall = append(fake.deleteQuotaArgsForCall, struct {
		name string
	}{name})
	fake.recordInvocation("DeleteQuota", []interface{}{name})
	fake.deleteQuotaMutex.Unlock()
	if fake.DeleteQuotaStub != nil {
		return fake.DeleteQuotaStub(name)
	} else {
		return fake.deleteQuotaReturns.result1
	}
}

func (fake *OrgDeleterCli) DeleteQuotaCallCount() int {
	fake.deleteQuotaMutex.RLock()
	defer fake.deleteQuotaMutex.RUnlock()
	return len(fake.deleteQuotaArgsForCall)
}

func (fake *OrgDeleterCli) DeleteQuotaArgsForCall(i int) string {
	fake.deleteQuotaMutex.RLock()
	defer fake.deleteQuotaMutex.RUnlock()
	return fake.deleteQuotaArgsForCall[i].name
}

func (fake *OrgDeleterCli) DeleteQuotaReturns(result1 error) {
	fake.DeleteQuotaStub = nil
	fake.deleteQuotaReturns = struct {
		result1 error
	}{result1}
}

func (fake *OrgDeleterCli) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.deleteOrgMutex.RLock()
	defer fake.deleteOrgMutex.RUnlock()
	fake.deleteQuotaMutex.RLock()
	defer fake.deleteQuotaMutex.RUnlock()
	return fake.invocations
}

func (fake *OrgDeleterCli) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}