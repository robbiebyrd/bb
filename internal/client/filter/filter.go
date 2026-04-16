package filter

import (
	"context"

	"github.com/robbiebyrd/bb/internal/client/common"
	canModels "github.com/robbiebyrd/bb/internal/models"
)

type FilterClient struct {
	ctx     *context.Context
	filter  canModels.CanMessageFilterGroup
	enabled bool
}

func NewFilterClient(ctx *context.Context) canModels.FilterInterface {
	return &FilterClient{
		ctx:     ctx,
		filter:  canModels.CanMessageFilterGroup{},
		enabled: true,
	}
}

func (fc *FilterClient) Filter(canMsg canModels.CanMessageTimestamped) bool {
	filterResults := []bool{}
	for _, f := range fc.filter.Filters {
		filterResults = append(filterResults, f.Filter(canMsg))
	}
	switch fc.filter.Operator {
	case canModels.FilterOr:
		return common.ArrayContainsTrue(filterResults)
	default:
		return common.ArrayAllTrue(filterResults)
	}
}

func (fc *FilterClient) Mode(mode canModels.CanFilterGroupOperator) {
	fc.filter.Operator = mode
}

func (fc *FilterClient) Add(filter canModels.CanMessageFilter) error {
	fc.filter.Filters = append(fc.filter.Filters, filter)
	return nil
}
