package resources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"

	"github.com/xing/terraform-provider-influxdb/internal/common"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &NotificationRuleResource{}
var _ resource.ResourceWithImportState = &NotificationRuleResource{}

func NewNotificationRuleResource() resource.Resource {
	return &NotificationRuleResource{}
}

// NotificationRuleResource defines the resource implementation.
type NotificationRuleResource struct {
	client     influxdb2.Client
	org        string
	serverURL  string
	authToken  string
	httpClient *http.Client
}

// NotificationRuleResourceModel describes the resource data model.
type NotificationRuleResourceModel struct {
	ID              types.String      `tfsdk:"id"`
	Name            types.String      `tfsdk:"name"`
	Org             types.String      `tfsdk:"org"`
	Description     types.String      `tfsdk:"description"`
	Status          types.String      `tfsdk:"status"`
	Type            types.String      `tfsdk:"type"`
	EndpointID      types.String      `tfsdk:"endpoint_id"`
	Every           types.String      `tfsdk:"every"`
	Offset          types.String      `tfsdk:"offset"`
	MessageTemplate types.String      `tfsdk:"message_template"`
	StatusRules     []StatusRuleModel `tfsdk:"status_rules"`
	TagRules        []TagRuleModel    `tfsdk:"tag_rules"`
}

type StatusRuleModel struct {
	CurrentLevel  types.String `tfsdk:"current_level"`
	PreviousLevel types.String `tfsdk:"previous_level"`
}

type TagRuleModel struct {
	Key      types.String `tfsdk:"key"`
	Value    types.String `tfsdk:"value"`
	Operator types.String `tfsdk:"operator"`
}

func (r *NotificationRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_rule"
}

func (r *NotificationRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "InfluxDB notification rule resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Notification rule ID",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Notification rule name",
			},
			"org": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Organization name or ID. If not provided, uses the provider default.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Notification rule description",
			},
			"status": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Status of the notification rule (active, inactive)",
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Type of the notification rule (http, slack, pagerduty)",
			},
			"endpoint_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the notification endpoint to send notifications to",
			},
			"every": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Check frequency (e.g., '1m', '5m')",
			},
			"offset": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Offset duration before checking",
			},
			"message_template": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Template for the notification message",
			},
		},
		Blocks: map[string]schema.Block{
			"status_rules": schema.ListNestedBlock{
				MarkdownDescription: "Rules based on check status levels",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"current_level": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Current status level (OK, INFO, WARN, CRIT)",
						},
						"previous_level": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Previous status level (OK, INFO, WARN, CRIT)",
						},
					},
				},
			},
			"tag_rules": schema.ListNestedBlock{
				MarkdownDescription: "Rules based on tag values",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Tag key",
						},
						"value": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Tag value",
						},
						"operator": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Operator for comparison (equal, notEqual, equalRegex, notEqualRegex)",
						},
					},
				},
			},
		},
	}
}

func (r *NotificationRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*common.ProviderData)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *common.ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = providerData.Client
	r.org = providerData.Org
	r.serverURL = providerData.URL
	r.authToken = providerData.Token
	r.httpClient = &http.Client{}
}

type StatusRule struct {
	CurrentLevel  string `json:"currentLevel"`
	PreviousLevel string `json:"previousLevel,omitempty"`
}

type TagRule struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Operator string `json:"operator"`
}

type NotificationRuleRequest struct {
	Name            string       `json:"name"`
	Description     *string      `json:"description,omitempty"`
	Status          string       `json:"status"`
	Type            string       `json:"type"`
	EndpointID      string       `json:"endpointID"`
	OwnerID         string       `json:"ownerID"`
	Every           string       `json:"every"`
	Offset          *string      `json:"offset,omitempty"`
	MessageTemplate *string      `json:"messageTemplate,omitempty"`
	StatusRules     []StatusRule `json:"statusRules"`
	TagRules        []TagRule    `json:"tagRules,omitempty"`
	OrgID           string       `json:"orgID"`
}

type NotificationRuleUpdateRequest struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Description     *string      `json:"description,omitempty"`
	Status          string       `json:"status"`
	Type            string       `json:"type"`
	EndpointID      string       `json:"endpointID"`
	OwnerID         string       `json:"ownerID"`
	Every           string       `json:"every"`
	Offset          *string      `json:"offset,omitempty"`
	MessageTemplate *string      `json:"messageTemplate,omitempty"`
	StatusRules     []StatusRule `json:"statusRules"`
	TagRules        []TagRule    `json:"tagRules,omitempty"`
	OrgID           string       `json:"orgID"`
}

type NotificationRuleResponse struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Description     *string      `json:"description"`
	Status          string       `json:"status"`
	Type            string       `json:"type"`
	EndpointID      string       `json:"endpointID"`
	Every           *string      `json:"every"`
	Offset          *string      `json:"offset"`
	MessageTemplate *string      `json:"messageTemplate"`
	StatusRules     []StatusRule `json:"statusRules"`
	TagRules        []TagRule    `json:"tagRules"`
	OrgID           string       `json:"orgID"`
}

func (r *NotificationRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data NotificationRuleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	org := r.org
	if !data.Org.IsNull() {
		org = data.Org.ValueString()
	}

	// Get org ID
	orgAPI := r.client.OrganizationsAPI()
	orgObj, err := orgAPI.FindOrganizationByName(ctx, org)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find organization %s, got error: %s", org, err))
		return
	}

	// Get the current user ID as the owner
	userAPI := r.client.UsersAPI()
	currentUser, err := userAPI.Me(ctx)
	if err != nil {
		resp.Diagnostics.AddError("[CREATE STAGE] User Error", fmt.Sprintf("Unable to get current user: %s", err))
		return
	}

	// Prepare request with values from model
	ruleReq := NotificationRuleRequest{
		Name:        data.Name.ValueString(),
		Status:      data.Status.ValueString(),
		Type:        data.Type.ValueString(),
		EndpointID:  data.EndpointID.ValueString(),
		OwnerID:     *currentUser.Id,
		Every:       data.Every.ValueString(),
		OrgID:       *orgObj.Id,
		StatusRules: []StatusRule{},
	}

	// Set offset from model
	offset := data.Offset.ValueString()
	ruleReq.Offset = &offset

	// Make HTTP request
	jsonData, err := json.Marshal(ruleReq)
	if err != nil {
		resp.Diagnostics.AddError("Serialization Error", fmt.Sprintf("Unable to serialize notification rule: %s", err))
		return
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v2/notificationRules", r.serverURL), bytes.NewBuffer(jsonData))
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create HTTP request: %s", err))
		return
	}

	httpReq.Header.Set("Authorization", "Token "+r.authToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("HTTP Error", fmt.Sprintf("Unable to create notification rule: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Response Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	if httpResp.StatusCode != http.StatusCreated {
		resp.Diagnostics.AddError("[CREATE STAGE] API Error", fmt.Sprintf("InfluxDB API returned status %d: %s", httpResp.StatusCode, string(body)))
		return
	}

	var rule NotificationRuleResponse
	if err := json.Unmarshal(body, &rule); err != nil {
		resp.Diagnostics.AddError("Deserialization Error", fmt.Sprintf("Unable to parse notification rule response: %s", err))
		return
	}

	// Update data with response
	data.ID = types.StringValue(rule.ID)
	data.Org = types.StringValue(org)
	data.Status = types.StringValue(rule.Status)
	data.Type = types.StringValue(rule.Type)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NotificationRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data NotificationRuleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Make HTTP request to get notification rule
	httpReq, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v2/notificationRules/%s", r.serverURL, data.ID.ValueString()), nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create HTTP request: %s", err))
		return
	}

	httpReq.Header.Set("Authorization", "Token "+r.authToken)
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("HTTP Error", fmt.Sprintf("Unable to read notification rule: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.Diagnostics.AddWarning("[READ STAGE] Resource Not Found", "Notification rule not found, removing from state")
		resp.State.RemoveResource(ctx)
		return
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Response Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("[READ STAGE] API Error", fmt.Sprintf("InfluxDB API returned status %d: %s", httpResp.StatusCode, string(body)))
		return
	}

	var rule NotificationRuleResponse
	if err := json.Unmarshal(body, &rule); err != nil {
		resp.Diagnostics.AddError("Deserialization Error", fmt.Sprintf("Unable to parse notification rule response: %s", err))
		return
	}

	// Update data with response
	data.ID = types.StringValue(rule.ID) // Ensure ID is preserved
	data.Name = types.StringValue(rule.Name)
	if rule.Description != nil {
		data.Description = types.StringValue(*rule.Description)
	}
	data.Status = types.StringValue(rule.Status)
	data.Type = types.StringValue(rule.Type)
	data.EndpointID = types.StringValue(rule.EndpointID)

	if rule.Every != nil {
		data.Every = types.StringValue(*rule.Every)
	}
	if rule.Offset != nil {
		data.Offset = types.StringValue(*rule.Offset)
	}
	if rule.MessageTemplate != nil {
		data.MessageTemplate = types.StringValue(*rule.MessageTemplate)
	}

	// Convert status rules
	if len(rule.StatusRules) > 0 {
		statusRules := make([]StatusRuleModel, len(rule.StatusRules))
		for i, rule := range rule.StatusRules {
			statusRules[i] = StatusRuleModel{
				CurrentLevel: types.StringValue(rule.CurrentLevel),
			}
			if rule.PreviousLevel != "" {
				statusRules[i].PreviousLevel = types.StringValue(rule.PreviousLevel)
			}
		}
		data.StatusRules = statusRules
	}

	// Convert tag rules
	if len(rule.TagRules) > 0 {
		tagRules := make([]TagRuleModel, len(rule.TagRules))
		for i, rule := range rule.TagRules {
			tagRules[i] = TagRuleModel{
				Key:      types.StringValue(rule.Key),
				Value:    types.StringValue(rule.Value),
				Operator: types.StringValue(rule.Operator),
			}
		}
		data.TagRules = tagRules
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NotificationRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data NotificationRuleResourceModel
	var state NotificationRuleResourceModel

	// Get the planned changes
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	// Get the current state to preserve ID and other computed fields
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Use ID from current state, not from plan
	if state.ID.IsNull() || state.ID.ValueString() == "" {
		resp.Diagnostics.AddError("[UPDATE STAGE] Missing ID", "Cannot update notification rule without an ID from current state")
		return
	}

	// Use the ID from the state
	data.ID = state.ID

	org := r.org
	if !data.Org.IsNull() {
		org = data.Org.ValueString()
	}

	// Get org ID
	orgAPI := r.client.OrganizationsAPI()
	orgObj, err := orgAPI.FindOrganizationByName(ctx, org)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find organization %s, got error: %s", org, err))
		return
	}

	// Get the current user ID as the owner
	userAPI := r.client.UsersAPI()
	currentUser, err := userAPI.Me(ctx)
	if err != nil {
		resp.Diagnostics.AddError("[UPDATE STAGE] User Error", fmt.Sprintf("Unable to get current user: %s", err))
		return
	}

	// Prepare request for PUT update (requires ID)
	ruleReq := NotificationRuleUpdateRequest{
		ID:          data.ID.ValueString(),
		Name:        data.Name.ValueString(),
		Status:      data.Status.ValueString(),
		Type:        data.Type.ValueString(),
		EndpointID:  data.EndpointID.ValueString(),
		OwnerID:     *currentUser.Id,
		Every:       data.Every.ValueString(),
		OrgID:       *orgObj.Id,
		StatusRules: []StatusRule{}, // Will be populated below if provided
	}

	// Set offset from model
	offset := data.Offset.ValueString()
	ruleReq.Offset = &offset

	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		ruleReq.Description = &desc
	}

	if !data.Offset.IsNull() {
		offset := data.Offset.ValueString()
		ruleReq.Offset = &offset
	}

	if !data.MessageTemplate.IsNull() {
		template := data.MessageTemplate.ValueString()
		ruleReq.MessageTemplate = &template
	}

	// Convert status rules
	if len(data.StatusRules) > 0 {
		statusRules := make([]StatusRule, len(data.StatusRules))
		for i, rule := range data.StatusRules {
			statusRules[i] = StatusRule{
				CurrentLevel: rule.CurrentLevel.ValueString(),
			}
			if !rule.PreviousLevel.IsNull() {
				statusRules[i].PreviousLevel = rule.PreviousLevel.ValueString()
			}
		}
		ruleReq.StatusRules = statusRules
	}

	// Convert tag rules
	if len(data.TagRules) > 0 {
		tagRules := make([]TagRule, len(data.TagRules))
		for i, rule := range data.TagRules {
			tagRules[i] = TagRule{
				Key:      rule.Key.ValueString(),
				Value:    rule.Value.ValueString(),
				Operator: rule.Operator.ValueString(),
			}
		}
		ruleReq.TagRules = tagRules
	}

	// Make HTTP request
	jsonData, err := json.Marshal(ruleReq)
	if err != nil {
		resp.Diagnostics.AddError("Serialization Error", fmt.Sprintf("Unable to serialize notification rule: %s", err))
		return
	}

	updateURL := fmt.Sprintf("%s/api/v2/notificationRules/%s", r.serverURL, data.ID.ValueString())
	httpReq, err := http.NewRequest("PUT", updateURL, bytes.NewBuffer(jsonData))
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create HTTP request for URL %s: %s", updateURL, err))
		return
	}

	httpReq.Header.Set("Authorization", "Token "+r.authToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	// Use default client like our working curl command
	httpResp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("HTTP Error", fmt.Sprintf("Unable to update notification rule: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("Response Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("[UPDATE STAGE] API Error", fmt.Sprintf("InfluxDB API returned status %d for URL %s with request body: %s\nResponse: %s", httpResp.StatusCode, updateURL, string(jsonData), string(body)))
		return
	}

	var rule NotificationRuleResponse
	if err := json.Unmarshal(body, &rule); err != nil {
		resp.Diagnostics.AddError("[UPDATE STAGE] Deserialization Error", fmt.Sprintf("Unable to parse notification rule response: %s\nResponse body: %s", err, string(body)))
		return
	}

	// Update data with response - preserve all current values and update what changed
	data.Name = types.StringValue(rule.Name)
	data.Status = types.StringValue(rule.Status)
	data.Type = types.StringValue(rule.Type)
	data.Org = types.StringValue(org) // Ensure org is properly set
	if rule.Every != nil {
		data.Every = types.StringValue(*rule.Every)
	}
	// Keep other fields as they are since they shouldn't change during update

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NotificationRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data NotificationRuleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Make HTTP request to delete notification rule
	httpReq, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v2/notificationRules/%s", r.serverURL, data.ID.ValueString()), nil)
	if err != nil {
		resp.Diagnostics.AddError("Request Error", fmt.Sprintf("Unable to create HTTP request: %s", err))
		return
	}

	httpReq.Header.Set("Authorization", "Token "+r.authToken)

	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("HTTP Error", fmt.Sprintf("Unable to delete notification rule: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusNoContent && httpResp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError("[DELETE STAGE] API Error", fmt.Sprintf("InfluxDB API returned status %d: %s", httpResp.StatusCode, string(body)))
		return
	}
}

func (r *NotificationRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
