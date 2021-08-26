// Copyright (c) 2021 Terminus, Inc.
//
// This program is free software: you can use, redistribute, and/or modify
// it under the terms of the GNU Affero General Public License, version 3
// or later ("AGPL"), as published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful, but WITHOUT
// ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
// FITNESS FOR A PARTICULAR PURPOSE.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package testplan_before

import (
	"errors"
	"strconv"
	"strings"

	"github.com/erda-project/erda-infra/base/servicehub"
	"github.com/erda-project/erda-proto-go/core/pipeline/cms/pb"
	"github.com/erda-project/erda/apistructs"
	"github.com/erda-project/erda/bundle"
	"github.com/erda-project/erda/modules/pipeline/providers/aop/aoptypes"
	"github.com/erda-project/erda/modules/pipeline/providers/aop/plugins_manage"
)

const (
	actionTypeAutoTestPlan = "auto-test-plan"
)

var providerInstance provider

type Plugin struct {
	aoptypes.PipelineBaseTunePoint
	CmsService pb.CmsServiceServer
	Bundle     *bundle.Bundle
}

func (p *Plugin) Name() string {
	return "testplan_before"
}

func (p *Plugin) Handle(ctx *aoptypes.TuneContext) error {
	// source = autotest
	if ctx.SDK.Pipeline.PipelineSource != apistructs.PipelineSourceAutoTest || ctx.SDK.Pipeline.IsSnippet {
		return nil
	}

	// PipelineYmlName is autotest-plan-xxx
	pipelineNamePre := apistructs.PipelineSourceAutoTestPlan.String() + "-"
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
	// get config from projectID
	var autoTestGlobalConfigListRequest apistructs.AutoTestGlobalConfigListRequest
	autoTestGlobalConfigListRequest.ScopeID = strconv.Itoa(int(testPlan.Data.ProjectID))
	autoTestGlobalConfigListRequest.Scope = "project-autotest-testcase"
	configs, err := p.Bundle.ListAutoTestGlobalConfigInternal(autoTestGlobalConfigListRequest)
	if err != nil {
		return err
	}

	var ns string
	meta := make(apistructs.PipelineReportMeta)
	for _, v := range configs {
		if v.Ns == ctx.SDK.Pipeline.PipelineExtra.Extra.ConfigManageNamespaces[0] {
			meta["data"] = v
			break
		}
	}
	if ns == "" {
		return errors.New("no found Pipeline NS")
	}

	// result 信息
	_, err = ctx.SDK.Report.Create(apistructs.PipelineReportCreateRequest{
		PipelineID: ctx.SDK.Pipeline.ID,
		Type:       actionTypeAutoTestPlan,
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
	providerInstance = *p
	for _, tuneTrigger := range p.Cfg.TuneTrigger {
		err := plugins_manage.RegisterTunePointToTuneGroup(p.Cfg.TuneType, tuneTrigger, New())
		if err != nil {
			panic(err)
		}
	}

	return nil
}

func init() {
	servicehub.Register("erda.core.pipeline.aop.plugins.pipeline.testplan-before", &servicehub.Spec{
		ConfigFunc: func() interface{} {
			return &config{}
		},
		Creator: func() servicehub.Provider {
			return &provider{}
		},
	})
}
