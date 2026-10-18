// Copyright (c) AO Studio
// SPDX-License-Identifier: MPL-2.0

package bundle

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestPlatformRequiresReplace(t *testing.T) {
	cases := []struct {
		state, plan string
		want        bool
	}{
		{state: "UNIVERSAL", plan: "IOS", want: false},
		{state: "UNIVERSAL", plan: "MAC_OS", want: false},
		{state: "IOS", plan: "MAC_OS", want: true},
		{state: "IOS", plan: "UNIVERSAL", want: true},
		{state: "MAC_OS", plan: "IOS", want: true},
	}

	for _, c := range cases {
		t.Run(c.state+"->"+c.plan, func(t *testing.T) {
			req := planmodifier.StringRequest{
				StateValue: types.StringValue(c.state),
				PlanValue:  types.StringValue(c.plan),
			}
			var resp stringplanmodifier.RequiresReplaceIfFuncResponse

			platformRequiresReplace(context.Background(), req, &resp)

			if resp.RequiresReplace != c.want {
				t.Errorf("RequiresReplace = %v, want %v", resp.RequiresReplace, c.want)
			}
		})
	}
}
