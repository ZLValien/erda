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
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/erda-project/erda/bundle"
	"github.com/erda-project/erda/modules/pipeline/dbclient"
	. "github.com/erda-project/erda/modules/pipeline/providers/aop/aoptypes"
	"github.com/erda-project/erda/modules/pipeline/services/reportsvc"
	"github.com/erda-project/erda/modules/pipeline/spec"
)

var (
	aopManage  = &AopManage{}
	pluginsMap = map[TuneType]map[string]TunePoint{}
)

type AopManage struct {
	// tuneGroup 保存所有 tune chain
	// 根据 类型、触发时机 初始化所有场景下的调用链
	tuneGroup   TuneGroup
	once        sync.Once
	initialized bool
	globalSDK   SDK
}

func InitAopManage() *AopManage {
	aopManage.once.Do(func() {
		db, err := dbclient.New()
		if err != nil {
			panic(err)
		}
		aopManage.initialized = true
		aopManage.globalSDK.Bundle = bundle.New(bundle.WithAllAvailableClients())
		aopManage.globalSDK.DBClient = db
		aopManage.globalSDK.Report = reportsvc.New(reportsvc.WithDBClient(db))
	})
	return aopManage
}

func (p *AopManage) Handle(ctx *TuneContext) error {
	if !p.initialized {
		return fmt.Errorf("AOP not initialized")
	}
	typ := ctx.SDK.TuneType
	trigger := ctx.SDK.TuneTrigger
	logrus.Debugf("AOP: type: %s, trigger: %s", typ, trigger)
	chain := p.tuneGroup.GetTuneChainByTypeAndTrigger(typ, trigger)
	if chain == nil || len(chain) == 0 {
		logrus.Debugf("AOP: type: %s, trigger: %s, tune chain is empty", typ, trigger)
		return nil
	}
	err := chain.Handle(ctx)
	if err != nil {
		logrus.Errorf("AOP: type: %s, trigger: %s, handle failed, err: %v", typ, trigger, err)
		return err
	}
	logrus.Debugf("AOP: type: %s, trigger: %s, handle success", typ, trigger)
	return nil
}

func (p *AopManage) ConvertTuneGroup(gp TunePointNameGroup) error {
	// Convert TunePointNameGroup to TuneGroup
	for tuneType := range gp {
		for tuneTrigger := range gp[tuneType] {
			for _, tuneName := range gp[tuneType][tuneTrigger] {
				err := registerTunePointToTuneGroup(tuneType, tuneTrigger, pluginsMap[tuneType][tuneName])
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func registerTunePointToTuneGroup(tuneType TuneType, tuneTrigger TuneTrigger, tunePoint TunePoint) error {
	if aopManage.tuneGroup == nil {
		aopManage.tuneGroup = make(map[TuneType]map[TuneTrigger]TuneChain)
	}

	group, ok := aopManage.tuneGroup[tuneType]
	if !ok {
		group = make(map[TuneTrigger]TuneChain)
	}
	group[tuneTrigger] = append(group[tuneTrigger], tunePoint)
	aopManage.tuneGroup[tuneType] = group
	return nil
}

// NewContextForPipeline 用于快速构造流水线 AOP 上下文
func (p *AopManage) NewContextForPipeline(pi spec.Pipeline, trigger TuneTrigger, customKVs ...map[interface{}]interface{}) *TuneContext {
	ctx := TuneContext{
		Context: context.Background(),
		SDK:     p.globalSDK.Clone(),
	}
	ctx.SDK.TuneType = TuneTypePipeline
	ctx.SDK.TuneTrigger = trigger
	ctx.SDK.Pipeline = pi
	// 用户自定义上下文
	for _, kvs := range customKVs {
		for k, v := range kvs {
			ctx.PutKV(k, v)
		}
	}
	return &ctx
}

// NewContextForTask 用于快速构任务 AOP 上下文
func (p *AopManage) NewContextForTask(task spec.PipelineTask, pi spec.Pipeline, trigger TuneTrigger, customKVs ...map[interface{}]interface{}) *TuneContext {
	// 先构造 pipeline 上下文
	ctx := p.NewContextForPipeline(pi, trigger, customKVs...)
	// 修改 tune type
	ctx.SDK.TuneType = TuneTypeTask
	// 注入特有 sdk 属性
	ctx.SDK.Task = task
	return ctx
}

func NewProviderNameByPluginName(tuneType TuneType, name string) string {
	return "erda.core.pipeline.aop.plugins." + string(tuneType) + "." + name
}