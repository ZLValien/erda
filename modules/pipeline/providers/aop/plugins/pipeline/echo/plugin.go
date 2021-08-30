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

package echo

import (
	"github.com/sirupsen/logrus"

	"github.com/erda-project/erda-infra/base/servicehub"
	"github.com/erda-project/erda/modules/pipeline/providers/aop"
	"github.com/erda-project/erda/modules/pipeline/providers/aop/aoptypes"
)

type Plugin struct {
	aoptypes.PipelineBaseTunePoint
}

func New() *Plugin { return &Plugin{} }

func (p *Plugin) Name() string { return "echo" }
func (p *Plugin) Handle(ctx *aoptypes.TuneContext) error {
	logrus.Debugf("say hello to pipeline AOP, type: %s, trigger: %s, pipelineID: %d, status: %s",
		ctx.SDK.TuneType, ctx.SDK.TuneTrigger, ctx.SDK.Pipeline.ID, ctx.SDK.Pipeline.Status)
	return nil
}

type config struct {
	TuneType    aoptypes.TuneType      `file:"tune_type"`
	TuneTrigger []aoptypes.TuneTrigger `file:"tune_trigger" `
}

// +provider
type provider struct {
	Cfg *config
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
