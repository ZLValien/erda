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

package testplan_after

import (
	"encoding/json"
	"fmt"
	"github.com/erda-project/erda/modules/pipeline/spec"
	"github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"

	"github.com/erda-project/erda-infra/base/servicehub"
	"github.com/erda-project/erda-proto-go/core/pipeline/cms/pb"
	"github.com/erda-project/erda/apistructs"
	"github.com/erda-project/erda/bundle"
	"github.com/erda-project/erda/modules/pipeline/providers/aop"
	"github.com/erda-project/erda/modules/pipeline/providers/aop/aoptypes"
)

const (
	version2 = "2.0"
)

var providerInstance provider

type ApiReportMeta struct {
	ApiTotalNum   int `json:"apiTotalNum"`
	ApiSuccessNum int `json:"apiSuccessNum"`
}

type Plugin struct {
	aoptypes.PipelineBaseTunePoint
	CmsService pb.CmsServiceServer
	Bundle     *bundle.Bundle
}

func (p *Plugin) Name() string {
	return "testplan-after"
}

func (p *Plugin) Handle(ctx *aoptypes.TuneContext) error {
	// source = autotest
	fmt.Println("!AOP: Start testplan_after,source:" + ctx.SDK.Pipeline.PipelineSource.String())
	if ctx.SDK.Pipeline.PipelineSource != apistructs.PipelineSourceAutoTest || ctx.SDK.Pipeline.IsSnippet {
		return nil
	}

	// PipelineYmlName is autotest-plan-xxx
	pipelineNamePre := apistructs.PipelineSourceAutoTestPlan.String() + "-"
	fmt.Println("!AOP: Start testplan_after,name:", pipelineNamePre)
	if !strings.HasPrefix(ctx.SDK.Pipeline.PipelineYmlName, pipelineNamePre) {
		return nil
	}
	testPlanIDStr := strings.TrimPrefix(ctx.SDK.Pipeline.PipelineYmlName, pipelineNamePre)
	testPlanID, err := strconv.ParseUint(testPlanIDStr, 10, 64)
	if err != nil {
		return err
	}

	var allTasks []*spec.PipelineTask
	// 尝试从上下文中获取，减少不必要的网络、数据库请求
	tasks, ok := ctx.TryGet(aoptypes.CtxKeyTasks)
	if ok {
		if _tasks, ok := tasks.([]*spec.PipelineTask); ok {
			allTasks = _tasks
		}
	} else {
		result, err := ctx.SDK.DBClient.GetPipelineWithTasks(ctx.SDK.Pipeline.ID)
		if err != nil {
			return err
		}
		allTasks = result.Tasks
	}
	// 过滤出 api_test task 以及 snippetTask
	var apiTestTasks []*spec.PipelineTask
	var snippetTaskPipelineIDs []uint64
	for _, task := range allTasks {
		if task.Type == apistructs.ActionTypeAPITest && task.Extra.Action.Version == "2.0" {
			apiTestTasks = append(apiTestTasks, task)
			continue
		}
		if task.Type == apistructs.ActionTypeSnippet {
			snippetTaskPipelineIDs = append(snippetTaskPipelineIDs, *task.SnippetPipelineID)
			continue
		}
	}

	apiTotalNum := 0
	apiSuccessNum := 0
	fmt.Println("!AOP: Start testplan_after,api:", apiTotalNum, "  ", apiSuccessNum)
	// snippetTask 从对应的 snippetPipeline api-test 报告里获取接口执行情况
	snippetReports, err := ctx.SDK.DBClient.BatchListPipelineReportsByPipelineID(
		snippetTaskPipelineIDs,
		[]string{string(apistructs.PipelineReportTypeAPITest)},
	)
	if err != nil {
		return err
	}

	for _, apiTestTask := range apiTestTasks {
		// 总数
		apiTotalNum++
		// 执行成功
		if apiTestTask.Status.IsSuccessStatus() {
			apiSuccessNum++
		}
	}
	for pipelineID, reports := range snippetReports {
		for _, report := range reports {
			b, err := json.Marshal(report.Meta)
			if err != nil {
				logrus.Warnf("failed to marshal api-test report, snippet pipelineID: %d, reportID: %d, err: %v",
					pipelineID, report.ID, err)
				continue
			}
			var meta ApiReportMeta
			if err := json.Unmarshal(b, &meta); err != nil {
				logrus.Warnf("failed to unmarshal api-test report to meta, snippet pipelineID: %d, reportID: %d, err: %v",
					pipelineID, report.ID, err)
				continue
			}
			// 总数
			apiTotalNum += meta.ApiTotalNum
			apiSuccessNum += meta.ApiSuccessNum
		}
	}

	var req = apistructs.TestPlanV2UpdateByHookRequest{
		TestPlanID:  testPlanID,
		PassRate:    "",
		ExecuteTime: ctx.SDK.Pipeline.ExtraTimeCreated,
	}

	if apiTotalNum == 0 {
		req.PassRate = "0.00%"
	} else {
		req.PassRate = strconv.FormatFloat(float64(apiSuccessNum)/float64(apiTotalNum)*100, 'f', 2, 64) + "%"
	}
	ccc, _ := json.Marshal(req)
	fmt.Println("!AOP: Start testplan_after,req: ", ccc)
	ev := &apistructs.EventCreateRequest{
		EventHeader: apistructs.EventHeader{
			Event:         bundle.AutoTestPlanEvent,
			Action:        bundle.UpdateAction,
			OrgID:         "-1",
			ProjectID:     "-1",
			ApplicationID: "-1",
			TimeStamp:     time.Now().Format("2006-01-02 15:04:05"),
		},
		Sender:  bundle.SenderDOP,
		Content: req,
	}
	// 发送应用更新事件
	if err = p.Bundle.CreateEvent(ev); err != nil {
		fmt.Println("failed to send application update event, (%v)", err)
	}
	return nil
}

func New() *Plugin {
	var p Plugin
	p.CmsService = providerInstance.CmsService
	p.Bundle = bundle.New(bundle.WithDOP())
	return &p
}

type config struct {
	TuneType    aoptypes.TuneType      `file:"tune_type"`
	TuneTrigger []aoptypes.TuneTrigger `file:"tune_trigger" `
}

// +provider
type provider struct {
	Cfg        *config
	CmsService pb.CmsServiceServer `autowired:"erda.core.pipeline.cms.CmsService"`
}

func (p *provider) Init(ctx servicehub.Context) error {
	err := aop.RegisterTunePoint(New())
	if err != nil {
		panic(err)
	}
	return nil
}

func init() {
	p := New()
	servicehub.Register(aop.NewProviderNameByPluginName(p.Type(), p.Name()), &servicehub.Spec{
		ConfigFunc: func() interface{} {
			return &config{}
		},
		Creator: func() servicehub.Provider {
			return &provider{}
		},
	})
}
