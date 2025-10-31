package filter

import (
	"context"

	"github.com/robbiebyrd/bb/internal/client/common"
	canModel "github.com/robbiebyrd/bb/internal/models"
)

type FilterInterface interface {
	Add(filter canModel.CanMessageFilter) error
	Filter(canMsg canModel.CanMessage) bool
	Mode(mode canModel.CanFilterGroupOperator)
}

type FilterClient struct {
	ctx    *context.Context
	filter canModel.CanMessageFilterGroup
}

func NewFilterClient(ctx *context.Context) *FilterClient {
	return &FilterClient{
		ctx:    ctx,
		filter: canModel.CanMessageFilterGroup{},
	}
}

func (fc *FilterClient) Filter(canMsg canModel.CanMessage) bool {
	filterResults := []bool{}
	for _, f := range fc.filter.Filters {
		filterResults = append(filterResults, f.Filter(canMsg))
	}
	switch fc.filter.Operator {
	case canModel.FilterOr:
		return common.ArrayContainsTrue(filterResults)
	default:
		return common.ArrayContainsFalse(filterResults)
	}
}

func (fc *FilterClient) Mode(mode canModel.CanFilterGroupOperator) {
	fc.filter.Operator = mode
}

func (fc *FilterClient) Add(filter canModel.CanMessageFilter) error {
	fc.filter.Filters = append(fc.filter.Filters, filter)
	return nil
}
