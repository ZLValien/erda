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

	"github.com/erda-project/erda-infra/base/logs"
	"github.com/erda-project/erda-infra/base/servicehub"
	"github.com/erda-project/erda/modules/pipeline/providers/aop/plugins_manage"
)

type config struct {
}

// +provider
type provider struct {
	Cfg *config
	Log logs.Logger

	PluginsManage *plugins_manage.PluginsManage
}

func (p *provider) Init(ctx servicehub.Context) error {
	pm := plugins_manage.InitPluginsManage()
	p.PluginsManage = pm
	return nil
}

func (p *provider) Run(ctx context.Context) error {
	return nil
}

func (p *provider) Provide(ctx servicehub.DependencyContext, args ...interface{}) interface{} {
	switch {
	case ctx.Service() == "erda.core.pipeline.aop.plugins":
		return p.PluginsManage
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
