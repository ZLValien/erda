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

package plugins_manage

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/erda-project/erda/apistructs"
	"github.com/erda-project/erda/modules/pipeline/providers/aop/aoptypes"
	"github.com/erda-project/erda/modules/pipeline/spec"
)

func init() {
	InitPluginsManage()
}

func TestHandlePipeline(t *testing.T) {
	ctx := pluginsManage.NewContextForTask(spec.PipelineTask{ID: 1}, spec.Pipeline{}, aoptypes.TuneTriggerTaskBeforeWait)
	err := pluginsManage.Handle(ctx)
	assert.NoError(t, err)

	// pipeline end
	ctx = pluginsManage.NewContextForPipeline(spec.Pipeline{
		PipelineBase: spec.PipelineBase{ID: 1},
	}, aoptypes.TuneTriggerPipelineAfterExec)
	err = pluginsManage.Handle(ctx)
	assert.NoError(t, err)
}

func TestHandleTask(t *testing.T) {
	// task end
	ctx := pluginsManage.NewContextForTask(
		spec.PipelineTask{ID: 1, Status: apistructs.PipelineStatusSuccess},
		spec.Pipeline{
			PipelineBase: spec.PipelineBase{ID: 1},
		}, aoptypes.TuneTriggerTaskAfterExec,
	)
	err := pluginsManage.Handle(ctx)
	assert.NoError(t, err)
}
