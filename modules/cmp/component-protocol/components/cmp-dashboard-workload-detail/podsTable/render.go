// Copyright (c) 2021 Terminus, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package podsTable

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/erda-project/erda-infra/base/servicehub"
	"github.com/erda-project/erda-infra/providers/component-protocol/cptype"
	"github.com/erda-project/erda-infra/providers/component-protocol/utils/cputil"
	"github.com/erda-project/erda/apistructs"
	"github.com/erda-project/erda/bundle"
	"github.com/erda-project/erda/modules/cmp/component-protocol/components/cmp-dashboard-pods/podsTable"
	cmpcputil "github.com/erda-project/erda/modules/cmp/component-protocol/cputil"
	"github.com/erda-project/erda/modules/cmp/component-protocol/types"
	"github.com/erda-project/erda/modules/cmp/metrics"
	"github.com/erda-project/erda/modules/openapi/component-protocol/components/base"
)

func init() {
	base.InitProviderWithCreator("cmp-dashboard-workload-detail", "podsTable", func() servicehub.Provider {
		return &ComponentPodsTable{}
	})
}

func (p *ComponentPodsTable) Render(ctx context.Context, component *cptype.Component, _ cptype.Scenario,
	event cptype.ComponentEvent, _ *cptype.GlobalStateData) error {
	p.InitComponent(ctx)
	if err := p.GenComponentState(component); err != nil {
		return fmt.Errorf("failed to gen podsTable component state, %v", err)
	}

	switch event.Operation {
	case cptype.InitializeOperation:
		p.State.PageNo = 1
		p.State.PageSize = 20
	case "changePageSize", "changeSort":
		p.State.PageNo = 1
	}

	if err := p.DecodeURLQuery(); err != nil {
		return fmt.Errorf("failed to decode url query for podsTable component, %v", err)
	}
	if err := p.RenderTable(); err != nil {
		return fmt.Errorf("failed to render podsTable component, %v", err)
	}
	if err := p.EncodeURLQuery(); err != nil {
		return fmt.Errorf("failed to encode url query for podsTable component, %v", err)
	}
	p.SetComponentValue(ctx)
	return nil
}

func (p *ComponentPodsTable) InitComponent(ctx context.Context) {
	bdl := ctx.Value(types.GlobalCtxKeyBundle).(*bundle.Bundle)
	p.bdl = bdl
	sdk := cputil.SDK(ctx)
	p.sdk = sdk
}

func (p *ComponentPodsTable) GenComponentState(c *cptype.Component) error {
	if c == nil || c.State == nil {
		return nil
	}
	var tableState State
	jsonData, err := json.Marshal(c.State)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(jsonData, &tableState); err != nil {
		return err
	}
	p.State = tableState
	return nil
}

func (p *ComponentPodsTable) DecodeURLQuery() error {
	urlQuery, ok := p.sdk.InParams["workloadTable__urlQuery"].(string)
	if !ok {
		return nil
	}
	decodeData, err := base64.StdEncoding.DecodeString(urlQuery)
	if err != nil {
		return err
	}
	query := make(map[string]interface{})
	if err := json.Unmarshal(decodeData, &query); err != nil {
		return err
	}
	p.State.PageNo = int(query["pageNo"].(float64))
	p.State.PageSize = int(query["pageSize"].(float64))
	sorterData := query["sorterData"].(map[string]interface{})
	p.State.Sorter.Field = sorterData["field"].(string)
	p.State.Sorter.Order = sorterData["order"].(string)
	return nil
}

func (p *ComponentPodsTable) EncodeURLQuery() error {
	urlQuery := make(map[string]interface{})
	urlQuery["pageNo"] = p.State.PageNo
	urlQuery["pageSize"] = p.State.PageSize
	urlQuery["sorterData"] = p.State.Sorter
	data, err := json.Marshal(urlQuery)
	if err != nil {
		return err
	}

	encodeData := base64.StdEncoding.EncodeToString(data)
	p.State.PodsTableURLQuery = encodeData
	return nil
}

func (p *ComponentPodsTable) RenderTable() error {
	userID := p.sdk.Identity.UserID
	orgID := p.sdk.Identity.OrgID
	workloadID := p.State.WorkloadID
	splits := strings.Split(workloadID, "_")
	if len(splits) != 3 {
		return fmt.Errorf("invalid workload id %s", workloadID)
	}
	kind, namespace, name := splits[0], splits[1], splits[2]

	req := apistructs.SteveRequest{
		UserID:      userID,
		OrgID:       orgID,
		Type:        apistructs.K8SResType(kind),
		ClusterName: p.State.ClusterName,
		Name:        name,
		Namespace:   namespace,
	}

	obj, err := p.bdl.GetSteveResource(&req)
	if err != nil {
		return err
	}

	labelSelectors := obj.Map("spec", "selector", "matchLabels")

	podReq := apistructs.SteveRequest{
		UserID:      userID,
		OrgID:       orgID,
		Type:        apistructs.K8SPod,
		ClusterName: p.State.ClusterName,
		Namespace:   namespace,
	}

	obj, err = p.bdl.ListSteveResource(&podReq)
	if err != nil {
		return err
	}
	list := obj.Slice("data")

	cpuReq := apistructs.MetricsRequest{
		UserID:       userID,
		OrgID:        orgID,
		ClusterName:  p.State.ClusterName,
		ResourceKind: metrics.Pod,
		ResourceType: metrics.Cpu,
	}
	memReq := apistructs.MetricsRequest{
		UserID:       userID,
		OrgID:        orgID,
		ClusterName:  p.State.ClusterName,
		ResourceKind: metrics.Pod,
		ResourceType: metrics.Memory,
	}

	tempCPULimits := make([]*resource.Quantity, 0)
	tempMemLimits := make([]*resource.Quantity, 0)
	var items []Item
	for _, obj := range list {
		labels := obj.Map("metadata", "labels")
		if !matchSelector(labelSelectors, labels) {
			continue
		}

		name := obj.String("metadata", "name")
		namespace := obj.String("metadata", "namespace")
		cpuReq.PodRequests = append(cpuReq.PodRequests, apistructs.MetricsPodRequest{
			PodName:   name,
			Namespace: namespace,
		})
		memReq.PodRequests = append(memReq.PodRequests, apistructs.MetricsPodRequest{
			PodName:   name,
			Namespace: namespace,
		})

		fields := obj.StringSlice("metadata", "fields")
		if len(fields) != 9 {
			logrus.Errorf("length of pod %s:%s fields is invalid", namespace, name)
			continue
		}
		status := p.parsePodStatus(fields[2])

		containers := obj.Slice("spec", "containers")
		cpuRequests := resource.NewQuantity(0, resource.DecimalSI)
		cpuLimits := resource.NewQuantity(0, resource.DecimalSI)
		memRequests := resource.NewQuantity(0, resource.BinarySI)
		memLimits := resource.NewQuantity(0, resource.BinarySI)
		for _, container := range containers {
			cpuRequests.Add(*parseResource(container.String("resources", "requests", "cpu"), resource.DecimalSI))
			cpuLimits.Add(*parseResource(container.String("resources", "limits", "cpu"), resource.DecimalSI))
			memRequests.Add(*parseResource(container.String("resources", "requests", "memory"), resource.BinarySI))
			memLimits.Add(*parseResource(container.String("resources", "limits", "memory"), resource.BinarySI))
		}

		cpuRequestStr := cmpcputil.ResourceToString(p.sdk, float64(cpuRequests.MilliValue()), resource.DecimalSI)
		if cpuRequests.MilliValue() == 0 {
			cpuRequestStr = "-"
		}
		cpuLimitsStr := cmpcputil.ResourceToString(p.sdk, float64(cpuLimits.MilliValue()), resource.DecimalSI)
		if cpuLimits.MilliValue() == 0 {
			cpuLimitsStr = "-"
		}
		memRequestsStr := cmpcputil.ResourceToString(p.sdk, float64(memRequests.Value()), resource.BinarySI)
		if memRequests.Value() == 0 {
			memRequestsStr = "-"
		}
		memLimitsStr := cmpcputil.ResourceToString(p.sdk, float64(memLimits.Value()), resource.BinarySI)
		if memLimits.Value() == 0 {
			memLimitsStr = "-"
		}

		tempCPULimits = append(tempCPULimits, cpuLimits)
		tempMemLimits = append(tempMemLimits, memLimits)

		id := fmt.Sprintf("%s_%s", namespace, name)
		items = append(items, Item{
			ID:     id,
			Status: status,
			Name: Link{
				RenderType: "linkText",
				Value:      name,
				Operations: map[string]interface{}{
					"click": LinkOperation{
						Command: Command{
							Key:    "goto",
							Target: "cmpClustersPodDetail",
							State: CommandState{
								Params: map[string]string{
									"podId": id,
								},
							},
							JumpOut: true,
						},
						Reload: false,
					},
				},
			},
			Namespace:         namespace,
			IP:                fields[5],
			CPURequests:       cpuRequestStr,
			CPURequestsNum:    cpuRequests.MilliValue(),
			CPULimits:         cpuLimitsStr,
			CPULimitsNum:      cpuLimits.MilliValue(),
			MemoryRequests:    memRequestsStr,
			MemoryRequestsNum: memRequests.Value(),
			MemoryLimits:      memLimitsStr,
			MemoryLimitsNum:   memLimits.Value(),
			Ready:             fields[1],
			NodeName:          fields[6],
		})
	}

	cpuMetrics, err := p.bdl.GetMetrics(cpuReq)
	if err != nil || len(cpuMetrics) == 0 {
		logrus.Errorf("failed to get cpu metrics for pods, %v", err)
		cpuMetrics = make([]apistructs.MetricsData, len(items), len(items))
	}
	memMetrics, err := p.bdl.GetMetrics(memReq)
	if err != nil || len(memMetrics) == 0 {
		logrus.Errorf("failed to get mem metrics for pods, %v", err)
		memMetrics = make([]apistructs.MetricsData, len(items), len(items))
	}

	for i := range items {
		cpuLimits := tempCPULimits[i]
		memLimits := tempMemLimits[i]

		cpuStatus, cpuValue, cpuTip := "success", "0", "N/A"
		usedCPUPercent := cpuMetrics[i].Used
		cpuStatus, cpuValue, cpuTip = p.parseResPercent(usedCPUPercent, cpuLimits, resource.DecimalSI)
		items[i].CPUPercent = Percent{
			RenderType: "progress",
			Value:      cpuValue,
			Tip:        cpuTip,
			Status:     cpuStatus,
		}

		memStatus, memValue, memTip := "success", "0", "N/A"
		usedMemPercent := memMetrics[i].Used
		memStatus, memValue, memTip = p.parseResPercent(usedMemPercent, memLimits, resource.BinarySI)
		items[i].MemoryPercent = Percent{
			RenderType: "progress",
			Value:      memValue,
			Tip:        memTip,
			Status:     memStatus,
		}
	}

	if p.State.Sorter.Field != "" {
		cmpWrapper := func(field, order string) func(int, int) bool {
			ascend := order == "ascend"
			switch field {
			case "status":
				return func(i int, j int) bool {
					less := items[i].Status.Value.Label < items[j].Status.Value.Label
					if ascend {
						return less
					}
					return !less
				}
			case "name":
				return func(i int, j int) bool {
					less := items[i].Name.Value < items[j].Name.Value
					if ascend {
						return less
					}
					return !less
				}
			case "namespace":
				return func(i int, j int) bool {
					less := items[i].Namespace < items[j].Namespace
					if ascend {
						return less
					}
					return !less
				}
			case "ip":
				return func(i int, j int) bool {
					less := items[i].IP < items[j].IP
					if ascend {
						return less
					}
					return !less
				}
			case "cpuRequests":
				return func(i int, j int) bool {
					less := items[i].CPURequestsNum < items[j].CPURequestsNum
					if ascend {
						return less
					}
					return !less
				}
			case "cpuPercent":
				return func(i int, j int) bool {
					vI, _ := strconv.ParseFloat(items[i].CPUPercent.Value, 64)
					vJ, _ := strconv.ParseFloat(items[j].CPUPercent.Value, 64)
					less := vI < vJ
					if ascend {
						return less
					}
					return !less
				}
			case "cpuLimits":
				return func(i int, j int) bool {
					less := items[i].CPULimitsNum < items[j].CPULimitsNum
					if ascend {
						return less
					}
					return !less
				}
			case "memoryRequests":
				return func(i int, j int) bool {
					less := items[i].MemoryRequestsNum < items[j].MemoryRequestsNum
					if ascend {
						return less
					}
					return !less
				}
			case "memoryPercent":
				return func(i int, j int) bool {
					vI, _ := strconv.ParseFloat(items[i].MemoryPercent.Value, 64)
					vJ, _ := strconv.ParseFloat(items[j].MemoryPercent.Value, 64)
					less := vI < vJ
					if ascend {
						return less
					}
					return !less
				}
			case "memoryLimits":
				return func(i int, j int) bool {
					less := items[i].MemoryLimitsNum < items[j].MemoryLimitsNum
					if ascend {
						return less
					}
					return !less
				}
			case "ready":
				return func(i int, j int) bool {
					splits := strings.Split(items[i].Ready, "/")
					readyI := splits[0]
					splits = strings.Split(items[j].Ready, "/")
					readyJ := splits[0]
					less := readyI < readyJ
					if ascend {
						return less
					}
					return !less
				}
			case "nodeName":
				return func(i int, j int) bool {
					less := items[i].NodeName < items[j].NodeName
					if ascend {
						return less
					}
					return !less
				}
			default:
				return func(i int, j int) bool {
					return false
				}
			}
		}
		sort.Slice(items, cmpWrapper(p.State.Sorter.Field, p.State.Sorter.Order))
	}

	l, r := getRange(len(items), p.State.PageNo, p.State.PageSize)
	p.Data.List = items[l:r]
	p.State.Total = len(items)
	return nil
}

func (p *ComponentPodsTable) parseResPercent(usedPercent float64, totQty *resource.Quantity, format resource.Format) (string, string, string) {
	var totRes int64
	if format == resource.DecimalSI {
		totRes = totQty.MilliValue()
	} else {
		totRes = totQty.Value()
	}
	usedRes := float64(totRes) * usedPercent / 100
	usedQtyString := cmpcputil.ResourceToString(p.sdk, usedRes, format)

	status := ""
	if usedPercent <= 80 {
		status = "success"
	} else if usedPercent < 100 {
		status = "warning"
	} else {
		status = "error"
	}

	tip := ""
	if format == resource.DecimalSI {
		tip = fmt.Sprintf("%s/%s", usedQtyString, cmpcputil.ResourceToString(p.sdk, float64(totQty.MilliValue()), format))
	} else {
		tip = fmt.Sprintf("%s/%s", usedQtyString, cmpcputil.ResourceToString(p.sdk, float64(totQty.Value()), format))
	}
	value := fmt.Sprintf("%.2f", usedPercent)
	if usedRes < 1e-8 {
		tip = "N/A"
		value = "N/A"
	}
	return status, value, tip
}

func (p *ComponentPodsTable) SetComponentValue(ctx context.Context) {
	p.Props.PageSizeOptions = []string{
		"10", "20", "50", "100",
	}
	p.Props.RowKey = "id"
	p.Props.Columns = []Column{
		{
			DataIndex: "status",
			Title:     cputil.I18n(ctx, "status"),
			Width:     80,
			Sorter:    true,
		},
		{
			DataIndex: "name",
			Title:     cputil.I18n(ctx, "name"),
			Width:     180,
			Sorter:    true,
		},
		{
			DataIndex: "namespace",
			Title:     cputil.I18n(ctx, "namespace"),
			Width:     180,
			Sorter:    true,
		},
		{
			DataIndex: "ip",
			Title:     cputil.I18n(ctx, "ip"),
			Width:     120,
			Sorter:    true,
		},
		{
			DataIndex: "cpuRequests",
			Title:     cputil.I18n(ctx, "cpuRequests"),
			Width:     120,
			Sorter:    true,
		},
		{
			DataIndex: "cpuLimits",
			Title:     cputil.I18n(ctx, "cpuLimits"),
			Width:     120,
			Sorter:    true,
		},
		{
			DataIndex: "cpuPercent",
			Title:     cputil.I18n(ctx, "cpuPercent"),
			Width:     120,
			Sorter:    true,
		},
		{
			DataIndex: "memoryRequests",
			Title:     cputil.I18n(ctx, "memoryRequests"),
			Width:     120,
			Sorter:    true,
		},
		{
			DataIndex: "memoryLimits",
			Title:     cputil.I18n(ctx, "memoryLimits"),
			Width:     120,
			Sorter:    true,
		},
		{
			DataIndex: "memoryPercent",
			Title:     cputil.I18n(ctx, "memoryPercent"),
			Width:     120,
			Sorter:    true,
		},
		{
			DataIndex: "ready",
			Title:     cputil.I18n(ctx, "ready"),
			Width:     80,
			Sorter:    true,
		},
		{
			DataIndex: "nodeName",
			Title:     cputil.I18n(ctx, "node"),
			Width:     120,
			Sorter:    true,
		},
	}

	p.Operations = map[string]interface{}{
		"changePageNo": Operation{
			Key:    "changePageNo",
			Reload: true,
		},
		"changePageSize": Operation{
			Key:    "changePageSize",
			Reload: true,
		},
		"changeSort": Operation{
			Key:    "changeSort",
			Reload: true,
		},
	}
}

func matchSelector(selector, labels map[string]interface{}) bool {
	for k, v := range selector {
		value, ok := v.(string)
		if !ok {
			return false
		}
		labelV, ok := labels[k]
		if !ok {
			return false
		}
		labelValue, ok := labelV.(string)
		if !ok || labelValue != value {
			return false
		}
	}
	return true
}

func (p *ComponentPodsTable) parsePodStatus(state string) Status {
	color := podsTable.PodStatusToColor[state]
	if color == "" {
		color = "darkslategray"
	}
	return Status{
		RenderType: "tagsRow",
		Size:       "default",
		Value: StatusValue{
			Label: p.sdk.I18n(state),
			Color: color,
		},
	}
}

func parseResource(resStr string, format resource.Format) *resource.Quantity {
	if resStr == "" {
		return resource.NewQuantity(0, format)
	}
	result, _ := resource.ParseQuantity(resStr)
	return &result
}

func getRange(length, pageNo, pageSize int) (int, int) {
	l := (pageNo - 1) * pageSize
	if l >= length || l < 0 {
		l = 0
	}
	r := l + pageSize
	if r > length || r < 0 {
		r = length
	}
	return l, r
}
