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

package aop

import (
	"context"
	"errors"

	"github.com/erda-project/erda-infra/base/logs"
	"github.com/erda-project/erda-infra/base/servicehub"
	"github.com/erda-project/erda/modules/pipeline/providers/aop/aoptypes"
	"github.com/erda-project/erda/modules/pipeline/spec"
)

var providerAOP *provider

type config struct {
	Chains aoptypes.TunePointNameGroup `file:"chains" env:"CHAINS" json:"chains"`
}

// +provider
type provider struct {
	Cfg *config
	Log logs.Logger

	AopManage *AopManage
}

func Handle(ctx *aoptypes.TuneContext) error {
	return providerAOP.AopManage.Handle(ctx)
}

func NewContextForTask(task spec.PipelineTask, pi spec.Pipeline, trigger aoptypes.TuneTrigger, customKVs ...map[interface{}]interface{}) *aoptypes.TuneContext {
	return providerAOP.AopManage.NewContextForTask(task, pi, trigger, customKVs...)
}
func NewContextForPipeline(pi spec.Pipeline, trigger aoptypes.TuneTrigger, customKVs ...map[interface{}]interface{}) *aoptypes.TuneContext {
	return providerAOP.AopManage.NewContextForPipeline(pi, trigger, customKVs...)
}

func RegisterTunePoint(tunePoint aoptypes.TunePoint) error {
	if _, ok := pluginsMap[tunePoint.Type()]; !ok {
		pluginsMap[tunePoint.Type()] = make(map[string]aoptypes.TunePoint)
	}

	// if tunePoint name is existed
	if _, ok := pluginsMap[tunePoint.Type()][tunePoint.Name()]; ok {
		return errors.New("tunePoint is existed, tuneType: " + string(tunePoint.Type()) + ", Name: " + tunePoint.Name())
	}
	pluginsMap[tunePoint.Type()][tunePoint.Name()] = tunePoint
	return nil
}

func (p *provider) Init(ctx servicehub.Context) error {
	p.AopManage = InitAopManage()
	providerAOP = p
	return nil
}

func (p *provider) Run(ctx context.Context) error {
	if err := p.AopManage.ConvertTuneGroup(p.Cfg.Chains); err != nil {
		return err
	}
	return nil
}

func (p *provider) Provide(ctx servicehub.DependencyContext, args ...interface{}) interface{} {
	switch {
	case ctx.Service() == "erda.core.pipeline.aop.plugins":
		return p.AopManage
	}
	return p
}

func init() {
	servicehub.Register("erda.core.pipeline.aop", &servicehub.Spec{
		Services: []string{"erda.core.pipeline.aop.plugins"},
		//Types:                pb.Types(),
		OptionalDependencies: []string{},
		Description:          "",
		ConfigFunc: func() interface{} {
			return &config{}
		},
		Creator: func() servicehub.Provider {
			return &provider{}
		},
	})
}
