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

package envHeaderTable

import (
	"context"
	"encoding/json"
	"github.com/erda-project/erda/apistructs"
	protocol "github.com/erda-project/erda/modules/openapi/component-protocol"
)

type ComponentFileInfo struct {
	CtxBdl protocol.ContextBundle

	CommonFileInfo
}

type CommonFileInfo struct {
	Version    string                                           `json:"version,omitempty"`
	Name       string                                           `json:"name,omitempty"`
	Type       string                                           `json:"type,omitempty"`
	Props      map[string]interface{}                           `json:"props,omitempty"`
	State      State                                            `json:"state,omitempty"`
	Operations map[apistructs.OperationKey]apistructs.Operation `json:"operations,omitempty"`
	Data       map[string]interface{}                           `json:"data,omitempty"`
}

type DataList struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type State struct {
	EnvData apistructs.AutoTestAPIConfig `json:"envData"`
	Value   []DataList                   `json:"value"`
}

func (a *ComponentFileInfo) Import(c *apistructs.Component) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, a); err != nil {
		return err
	}
	return nil
}

func (i *ComponentFileInfo) RenderProtocol(c *apistructs.Component, g *apistructs.GlobalStateData) {
	c.Props = i.Props
	c.State = map[string]interface{}{
		"value": i.State.Value,
	}
}

func (i *ComponentFileInfo) Render(ctx context.Context, c *apistructs.Component, _ apistructs.ComponentProtocolScenario, event apistructs.ComponentEvent, gs *apistructs.GlobalStateData) (err error) {
	if err := i.Import(c); err != nil {
		return err
	}

	i.CtxBdl = ctx.Value(protocol.GlobalInnerKeyCtxBundle.String()).(protocol.ContextBundle)

	i.Props = map[string]interface{}{
		"readOnly": true,
		"actions": map[string]interface{}{
			"copy":   true,
			"format": true,
		},
	}

	var list []DataList
	for k, v := range i.State.EnvData.Header {
		list = append(list, DataList{
			Name:    k,
			Content: v,
		})
	}
	i.State.Value = list
	i.RenderProtocol(c, gs)
	return
}

func RenderCreator() protocol.CompRender {
	return &ComponentFileInfo{
		CtxBdl: protocol.ContextBundle{},
		CommonFileInfo: CommonFileInfo{
			Props:      map[string]interface{}{},
			Operations: map[apistructs.OperationKey]apistructs.Operation{},
			Data:       map[string]interface{}{},
		},
	}
}
