// Code generated by pegomock. DO NOT EDIT.
// Source: io (interfaces: ReadCloser)

package bitsgo_test

import (
	pegomock "github.com/petergtz/pegomock"
	"reflect"
)

type MockReadCloser struct {
	fail func(message string, callerSkip ...int)
}

func NewMockReadCloser() *MockReadCloser {
	return &MockReadCloser{fail: pegomock.GlobalFailHandler}
}

func (mock *MockReadCloser) Close() error {
	params := []pegomock.Param{}
	result := pegomock.GetGenericMockFrom(mock).Invoke("Close", params, []reflect.Type{reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(error)
		}
	}
	return ret0
}

func (mock *MockReadCloser) Read(_param0 []byte) (int, error) {
	params := []pegomock.Param{_param0}
	result := pegomock.GetGenericMockFrom(mock).Invoke("Read", params, []reflect.Type{reflect.TypeOf((*int)(nil)).Elem(), reflect.TypeOf((*error)(nil)).Elem()})
	var ret0 int
	var ret1 error
	if len(result) != 0 {
		if result[0] != nil {
			ret0 = result[0].(int)
		}
		if result[1] != nil {
			ret1 = result[1].(error)
		}
	}
	return ret0, ret1
}

func (mock *MockReadCloser) VerifyWasCalledOnce() *VerifierReadCloser {
	return &VerifierReadCloser{mock, pegomock.Times(1), nil}
}

func (mock *MockReadCloser) VerifyWasCalled(invocationCountMatcher pegomock.Matcher) *VerifierReadCloser {
	return &VerifierReadCloser{mock, invocationCountMatcher, nil}
}

func (mock *MockReadCloser) VerifyWasCalledInOrder(invocationCountMatcher pegomock.Matcher, inOrderContext *pegomock.InOrderContext) *VerifierReadCloser {
	return &VerifierReadCloser{mock, invocationCountMatcher, inOrderContext}
}

type VerifierReadCloser struct {
	mock                   *MockReadCloser
	invocationCountMatcher pegomock.Matcher
	inOrderContext         *pegomock.InOrderContext
}

func (verifier *VerifierReadCloser) Close() *ReadCloser_Close_OngoingVerification {
	params := []pegomock.Param{}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "Close", params)
	return &ReadCloser_Close_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type ReadCloser_Close_OngoingVerification struct {
	mock              *MockReadCloser
	methodInvocations []pegomock.MethodInvocation
}

func (c *ReadCloser_Close_OngoingVerification) GetCapturedArguments() {
}

func (c *ReadCloser_Close_OngoingVerification) GetAllCapturedArguments() {
}

func (verifier *VerifierReadCloser) Read(_param0 []byte) *ReadCloser_Read_OngoingVerification {
	params := []pegomock.Param{_param0}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "Read", params)
	return &ReadCloser_Read_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type ReadCloser_Read_OngoingVerification struct {
	mock              *MockReadCloser
	methodInvocations []pegomock.MethodInvocation
}

func (c *ReadCloser_Read_OngoingVerification) GetCapturedArguments() []byte {
	_param0 := c.GetAllCapturedArguments()
	return _param0[len(_param0)-1]
}

func (c *ReadCloser_Read_OngoingVerification) GetAllCapturedArguments() (_param0 [][]byte) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([][]byte, len(params[0]))
		for u, param := range params[0] {
			_param0[u] = param.([]byte)
		}
	}
	return
}
