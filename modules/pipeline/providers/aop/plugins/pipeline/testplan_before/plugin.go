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

package testplan_before

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/erda-project/erda-infra/base/servicehub"
	"github.com/erda-project/erda-proto-go/core/pipeline/cms/pb"
	"github.com/erda-project/erda/apistructs"
	"github.com/erda-project/erda/bundle"
	"github.com/erda-project/erda/modules/pipeline/providers/aop"
	"github.com/erda-project/erda/modules/pipeline/providers/aop/aoptypes"
)

var providerInstance provider

type Plugin struct {
	aoptypes.PipelineBaseTunePoint
	CmsService pb.CmsServiceServer
	Bundle     *bundle.Bundle
}

func (p *Plugin) Name() string {
	return "testplan-before"
}

func (p *Plugin) Handle(ctx *aoptypes.TuneContext) error {
	fmt.Println("!AOP before: ", ctx.SDK.Pipeline.PipelineSource)
	// source = autotest
	if ctx.SDK.Pipeline.PipelineSource != apistructs.PipelineSourceAutoTest || ctx.SDK.Pipeline.IsSnippet {
		return nil
	}

	// PipelineYmlName is autotest-plan-xxx
	pipelineNamePre := apistructs.PipelineSourceAutoTestPlan.String() + "-"
	if !strings.HasPrefix(ctx.SDK.Pipeline.PipelineYmlName, pipelineNamePre) {
		return nil
	}
	testPlanIDStr := strings.TrimPrefix(ctx.SDK.Pipeline.PipelineYmlName, pipelineNamePre)
	testPlanID, err := strconv.ParseUint(testPlanIDStr, 10, 64)
	if err != nil {
		return err
	}

	// get TestPlan
	testPlan, err := p.Bundle.GetTestPlanV2(testPlanID)
	if err != nil {
		return err
	}
	fmt.Println("!AOP config: ", testPlan)
	// get config from projectID
	var autoTestGlobalConfigListRequest apistructs.AutoTestGlobalConfigListRequest
	autoTestGlobalConfigListRequest.ScopeID = strconv.Itoa(int(testPlan.Data.ProjectID))
	autoTestGlobalConfigListRequest.Scope = "project-autotest-testcase"
	autoTestGlobalConfigListRequest.UserID = ctx.SDK.Pipeline.PipelineExtra.Snapshot.PlatformSecrets["dice.user.id"]
	configs, err := p.Bundle.ListAutoTestGlobalConfig(autoTestGlobalConfigListRequest)
	if err != nil {
		return err
	}

	fmt.Println("!AOP config: ", ctx.SDK.Pipeline.PipelineSource)
	meta := make(apistructs.PipelineReportMeta)
	for _, v := range configs {
		if v.Ns == ctx.SDK.Pipeline.PipelineExtra.Extra.ConfigManageNamespaces[0] {
			meta["data"] = v
			break
		}
	}
	if _, ok := meta["data"]; !ok {
		return errors.New("no found Pipeline NS")
	}

	fmt.Println("!AOP send:::! ")

	// result 信息
	_, err = ctx.SDK.Report.Create(apistructs.PipelineReportCreateRequest{
		PipelineID: ctx.SDK.Pipeline.ID,
		Type:       apistructs.PipelineReportTypeAutotestPlan,
		Meta:       meta,
	})
	if err != nil {
		return err
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
