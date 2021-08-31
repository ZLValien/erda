package autotestv2

import (
	"github.com/alecthomas/assert"
	"github.com/erda-project/erda/apistructs"
	"github.com/erda-project/erda/bundle"
	"testing"
)

func Test_1(t *testing.T) {
	bdl := bundle.New(bundle.WithDOP())
	testplan, err := bdl.GetTestPlanV2(4)
	assert.NoError(t, err)
	err = bdl.UpdateTestPlanV2(apistructs.TestPlanV2UpdateRequest{
		Name:         testplan.Data.Name,
		Desc:         testplan.Data.Desc,
		SpaceID:      testplan.Data.SpaceID,
		Owners:       testplan.Data.Owners,
		State:        apistructs.TestPlanV2StateArchived,
		TestPlanID:   testplan.Data.ID,
		IdentityInfo: apistructs.IdentityInfo{UserID: "2"},
	})
	assert.NoError(t, err)
}
