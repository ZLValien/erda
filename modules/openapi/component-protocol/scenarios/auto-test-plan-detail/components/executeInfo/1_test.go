package executeInfo

import (
	"encoding/json"
	"github.com/alecthomas/assert"
	"github.com/erda-project/erda/apistructs"
	"testing"
)

func Test_1(t *testing.T) {
	m := apistructs.PipelineReportMeta{}
	bt := `{"data":{"scope":"project-autotest-testcase","scopeID":"6","ns":"autotest^scope-project-autotest-testcase^scopeid-6^369797316552438452","displayName":"a","desc":"1","creatorID":"2","updaterID":"","apiConfig":{"domain":"https://openapi.hkci.terminus.io","header":{"Cookie":"u_c_captain_hkci_local=eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJsb2dpbiIsInBhdGgiOiIvIiwidG9rZW5LZXkiOiI5NTA4NjI2ZTczZDdiODdjYjY1N2M3YTUzMGY1NWM4ZDg0OTJjYjc0YjFmZDdlZTNlNjhmZThkNThmZjg4ZDM2IiwibmJmIjoxNjI5ODkwMTMyLCJkb21haW4iOiJ0ZXJtaW51cy5pbyIsImlzcyI6ImRyYWNvIiwidGVuYW50SWQiOjEsImV4cGlyZV90aW1lIjo2MDQ4MDAsImV4cCI6MTYzMDQ5NDkzMiwiaWF0IjoxNjI5ODkwMTMyfQ.jTI2D54sC-aM90p8TX9pfeXXwOFi8Yns6smQhNSVZKA; OPENAPISESSION=6cccd9ac-38f0-4906-9da8-0143186dd631; OPENAPI-CSRF-TOKEN=12bde30a9db3d9aad0159ecbd9ae79b89005c3a5eca326a2b7d0f349cf897441342e1bc368ab501084ed6de62d8e57ca","cluster-id":"2","cluster-name":"ZXJkYS1ob25na29uZw==","org":"erda","project-id":"13"},"global":{"111":{"name":"111","type":"string","value":"111","desc":"111"}}}}}`
	err := json.Unmarshal([]byte(bt), &m)
	assert.NoError(t, err)
	c, err := convertReportToConfig(apistructs.PipelineReport{
		Meta: m,
	})
	assert.NoError(t, err)
	want := apistructs.AutoTestGlobalConfig{
		Scope:       "project-autotest-testcase",
		ScopeID:     "6",
		Ns:          "autotest^scope-project-autotest-testcase^scopeid-6^369797316552438452",
		DisplayName: "a",
		Desc:        "1",
		CreatorID:   "2",
		UpdaterID:   "",
		CreatedAt:   nil,
		UpdatedAt:   nil,
		APIConfig: &apistructs.AutoTestAPIConfig{
			Domain: "https://openapi.hkci.terminus.io",
			Header: map[string]string{
				"Cookie":       "u_c_captain_hkci_local=eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJsb2dpbiIsInBhdGgiOiIvIiwidG9rZW5LZXkiOiI5NTA4NjI2ZTczZDdiODdjYjY1N2M3YTUzMGY1NWM4ZDg0OTJjYjc0YjFmZDdlZTNlNjhmZThkNThmZjg4ZDM2IiwibmJmIjoxNjI5ODkwMTMyLCJkb21haW4iOiJ0ZXJtaW51cy5pbyIsImlzcyI6ImRyYWNvIiwidGVuYW50SWQiOjEsImV4cGlyZV90aW1lIjo2MDQ4MDAsImV4cCI6MTYzMDQ5NDkzMiwiaWF0IjoxNjI5ODkwMTMyfQ.jTI2D54sC-aM90p8TX9pfeXXwOFi8Yns6smQhNSVZKA; OPENAPISESSION=6cccd9ac-38f0-4906-9da8-0143186dd631; OPENAPI-CSRF-TOKEN=12bde30a9db3d9aad0159ecbd9ae79b89005c3a5eca326a2b7d0f349cf897441342e1bc368ab501084ed6de62d8e57ca",
				"cluster-id":   "2",
				"cluster-name": "ZXJkYS1ob25na29uZw==",
				"org":          "erda",
				"project-id":   "13",
			},
			Global: map[string]apistructs.AutoTestConfigItem{
				"111": {
					Name:  "111",
					Type:  "string",
					Value: "111",
					Desc:  "111",
				},
			},
		},
		UIConfig: nil,
	}
	assert.Equal(t, want, c)
}
