package testplan_before

import (
	"github.com/alecthomas/assert"
	"github.com/erda-project/erda/apistructs"
	"github.com/erda-project/erda/modules/pipeline/providers/aop/aoptypes"
	"testing"
)

func Test_1(t *testing.T) {
	p := Plugin{}
	tu := &aoptypes.TuneContext{}
	tu.SDK.Pipeline.IsSnippet = false
	tu.SDK.Pipeline.PipelineSource = apistructs.PipelineSourceAutoTest
	tu.SDK.Pipeline.PipelineYmlName = apistructs.PipelineSourceAutoTestPlan.String() + "-" + "4"
	e := p.Handle(tu)
	assert.NoError(t, e)
}
