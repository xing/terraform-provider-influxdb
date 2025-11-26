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
var _ resource.Resource = &NotificationEndpointResource{}
var _ resource.ResourceWithImportState = &NotificationEndpointResource{}

func NewNotificationEndpointResource() resource.Resource {
	return &NotificationEndpointResource{}
}

// NotificationEndpointResource defines the resource implementation.
type NotificationEndpointResource struct {
	client     influxdb2.Client
	org        string
	serverURL  string
	authToken  string
	httpClient *http.Client
}

// NotificationEndpointResourceModel describes the resource data model.
type NotificationEndpointResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Org         types.String `tfsdk:"org"`
	Description types.String `tfsdk:"description"`
	Status      types.String `tfsdk:"status"`
	Type        types.String `tfsdk:"type"`
	URL         types.String `tfsdk:"url"`
	Token       types.String `tfsdk:"token"`
	Username    types.String `tfsdk:"username"`
	Password    types.String `tfsdk:"password"`
	Method      types.String `tfsdk:"method"`
	AuthMethod  types.String `tfsdk:"auth_method"`
	Headers     types.Map    `tfsdk:"headers"`
}

func (r *NotificationEndpointResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_endpoint"
}

func (r *NotificationEndpointResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "InfluxDB notification endpoint resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Notification endpoint ID",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Notification endpoint name",
			},
			"org": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Organization name or ID. If not provided, uses the provider default.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Notification endpoint description",
			},
			"status": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Status of the notification endpoint (active, inactive)",
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Type of notification endpoint (http, slack, pagerduty, etc.)",
			},
			"url": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "URL of the notification endpoint",
			},
			"token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Authentication token (for endpoints that require it)",
			},
			"username": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Username for basic authentication",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "Password for basic authentication",
			},
			"method": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "HTTP method to use (POST, PUT, etc.)",
			},
			"auth_method": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Authentication method (none, basic, bearer)",
			},
			"headers": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Additional headers to send with the request",
			},
		},
	}
}

func (r *NotificationEndpointResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

type NotificationEndpointRequest struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	URL        string `json:"url"`
	Status     string `json:"status"`
	Method     string `json:"method"`
	AuthMethod string `json:"authMethod"`
	OrgID      string `json:"orgID"`
}

type NotificationEndpointResponse struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description *string           `json:"description"`
	Status      string            `json:"status"`
	Type        string            `json:"type"`
	URL         string            `json:"url"`
	Token       *string           `json:"token"`
	Username    *string           `json:"username"`
	Password    *string           `json:"password"`
	Method      string            `json:"method"`
	AuthMethod  string            `json:"auth_method"`
	Headers     map[string]string `json:"headers"`
	OrgID       string            `json:"orgID"`
}

func (r *NotificationEndpointResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data NotificationEndpointResourceModel

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
		resp.Diagnostics.AddError("[CREATE STAGE] Client Error", fmt.Sprintf("Unable to find organization %s, got error: %s", org, err))
		return
	}

	endpointReq := NotificationEndpointRequest{
		Name:       data.Name.ValueString(),
		Type:       data.Type.ValueString(),
		URL:        data.URL.ValueString(),
		Status:     "active",
		Method:     "POST",
		AuthMethod: "none",
		OrgID:      *orgObj.Id,
	}

	// Make HTTP request
	jsonData, err := json.Marshal(endpointReq)
	if err != nil {
		resp.Diagnostics.AddError("[CREATE STAGE] Serialization Error", fmt.Sprintf("Unable to serialize notification endpoint: %s", err))
		return
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v2/notificationEndpoints", r.serverURL), bytes.NewBuffer(jsonData))
	if err != nil {
		resp.Diagnostics.AddError("[CREATE STAGE] Request Error", fmt.Sprintf("Unable to create HTTP request: %s", err))
		return
	}

	httpReq.Header.Set("Authorization", "Token "+r.authToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("[CREATE STAGE] HTTP Error", fmt.Sprintf("Unable to create notification endpoint: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("[CREATE STAGE] Response Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	if httpResp.StatusCode != http.StatusCreated {
		resp.Diagnostics.AddError("[CREATE STAGE] API Error", fmt.Sprintf("InfluxDB API returned status %d: %s", httpResp.StatusCode, string(body)))
		return
	}

	var endpoint NotificationEndpointResponse
	if err := json.Unmarshal(body, &endpoint); err != nil {
		resp.Diagnostics.AddError("[CREATE STAGE] Deserialization Error", fmt.Sprintf("Unable to parse notification endpoint response: %s", err))
		return
	}

	// Update data with response
	data.ID = types.StringValue(endpoint.ID)
	data.Org = types.StringValue(org)
	data.Status = types.StringValue(endpoint.Status)
	data.Method = types.StringValue(endpoint.Method)
	data.AuthMethod = types.StringValue(endpoint.AuthMethod)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NotificationEndpointResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data NotificationEndpointResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Make HTTP request to get notification endpoint
	httpReq, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v2/notificationEndpoints/%s", r.serverURL, data.ID.ValueString()), nil)
	if err != nil {
		resp.Diagnostics.AddError("[READ STAGE] Request Error", fmt.Sprintf("Unable to create HTTP request: %s", err))
		return
	}

	httpReq.Header.Set("Authorization", "Token "+r.authToken)
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("[READ STAGE] HTTP Error", fmt.Sprintf("Unable to read notification endpoint: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.Diagnostics.AddWarning("[READ STAGE] Resource Not Found", "Notification endpoint not found, removing from state")
		resp.State.RemoveResource(ctx)
		return
	}

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("[READ STAGE] Response Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("[READ STAGE] API Error", fmt.Sprintf("InfluxDB API returned status %d: %s", httpResp.StatusCode, string(body)))
		return
	}

	var endpoint NotificationEndpointResponse
	if err := json.Unmarshal(body, &endpoint); err != nil {
		resp.Diagnostics.AddError("[READ STAGE] Deserialization Error", fmt.Sprintf("Unable to parse notification endpoint response: %s", err))
		return
	}

	// Update data with response
	data.Name = types.StringValue(endpoint.Name)
	if endpoint.Description != nil {
		data.Description = types.StringValue(*endpoint.Description)
	}
	data.Status = types.StringValue(endpoint.Status)
	data.Type = types.StringValue(endpoint.Type)
	data.URL = types.StringValue(endpoint.URL)
	data.Method = types.StringValue(endpoint.Method)
	if endpoint.AuthMethod != "" {
		data.AuthMethod = types.StringValue(endpoint.AuthMethod)
	}

	if len(endpoint.Headers) > 0 {
		headers, diags := types.MapValueFrom(ctx, types.StringType, endpoint.Headers)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Headers = headers
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NotificationEndpointResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data NotificationEndpointResourceModel

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
		resp.Diagnostics.AddError("[UPDATE STAGE] Client Error", fmt.Sprintf("Unable to find organization %s, got error: %s", org, err))
		return
	}

	// Prepare request with user-provided values
	endpointReq := NotificationEndpointRequest{
		Name:       data.Name.ValueString(),
		Type:       data.Type.ValueString(),
		URL:        data.URL.ValueString(),
		Status:     "active", // Default to active
		Method:     "POST",   // Default method
		AuthMethod: "none",   // Default auth method
		OrgID:      *orgObj.Id,
	}

	// Override with user-provided values if specified
	if !data.Status.IsNull() {
		endpointReq.Status = data.Status.ValueString()
	}
	if !data.Method.IsNull() {
		endpointReq.Method = data.Method.ValueString()
	}
	if !data.AuthMethod.IsNull() {
		endpointReq.AuthMethod = data.AuthMethod.ValueString()
	}

	// Make HTTP request
	jsonData, err := json.Marshal(endpointReq)
	if err != nil {
		resp.Diagnostics.AddError("[UPDATE STAGE] Serialization Error", fmt.Sprintf("Unable to serialize notification endpoint: %s", err))
		return
	}

	httpReq, err := http.NewRequest("PATCH", fmt.Sprintf("%s/api/v2/notificationEndpoints/%s", r.serverURL, data.ID.ValueString()), bytes.NewBuffer(jsonData))
	if err != nil {
		resp.Diagnostics.AddError("[UPDATE STAGE] Request Error", fmt.Sprintf("Unable to create HTTP request: %s", err))
		return
	}

	httpReq.Header.Set("Authorization", "Token "+r.authToken)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("[UPDATE STAGE] HTTP Error", fmt.Sprintf("Unable to update notification endpoint: %s", err))
		return
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		resp.Diagnostics.AddError("[UPDATE STAGE] Response Error", fmt.Sprintf("Unable to read response body: %s", err))
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("[UPDATE STAGE] API Error", fmt.Sprintf("InfluxDB API returned status %d: %s", httpResp.StatusCode, string(body)))
		return
	}

	var endpoint NotificationEndpointResponse
	if err := json.Unmarshal(body, &endpoint); err != nil {
		resp.Diagnostics.AddError("[UPDATE STAGE] Deserialization Error", fmt.Sprintf("Unable to parse notification endpoint response: %s", err))
		return
	}

	// Update data with response
	data.Status = types.StringValue(endpoint.Status)
	data.Method = types.StringValue(endpoint.Method)
	data.AuthMethod = types.StringValue(endpoint.AuthMethod)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NotificationEndpointResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data NotificationEndpointResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Make HTTP request to delete notification endpoint
	httpReq, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v2/notificationEndpoints/%s", r.serverURL, data.ID.ValueString()), nil)
	if err != nil {
		resp.Diagnostics.AddError("[DELETE STAGE] Request Error", fmt.Sprintf("Unable to create HTTP request: %s", err))
		return
	}

	httpReq.Header.Set("Authorization", "Token "+r.authToken)

	httpResp, err := r.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("[DELETE STAGE] HTTP Error", fmt.Sprintf("Unable to delete notification endpoint: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusNoContent && httpResp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError("[DELETE STAGE] API Error", fmt.Sprintf("InfluxDB API returned status %d: %s", httpResp.StatusCode, string(body)))
		return
	}
}

func (r *NotificationEndpointResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
