// Code generated by mockery v2.14.0. DO NOT EDIT.

package mocks

import (
	context "context"

	types "github.com/palomachain/pigeon/types/paloma/x/scheduler/types"
	mock "github.com/stretchr/testify/mock"
)

// QueryServer is an autogenerated mock type for the QueryServer type
type QueryServer struct {
	mock.Mock
}

// Params provides a mock function with given fields: _a0, _a1
func (_m *QueryServer) Params(_a0 context.Context, _a1 *types.QueryParamsRequest) (*types.QueryParamsResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *types.QueryParamsResponse
	if rf, ok := ret.Get(0).(func(context.Context, *types.QueryParamsRequest) *types.QueryParamsResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.QueryParamsResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *types.QueryParamsRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewQueryServer interface {
	mock.TestingT
	Cleanup(func())
}

// NewQueryServer creates a new instance of QueryServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewQueryServer(t mockConstructorTestingTNewQueryServer) *QueryServer {
	mock := &QueryServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
