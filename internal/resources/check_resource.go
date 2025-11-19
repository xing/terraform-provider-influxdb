package resources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &CheckResource{}
var _ resource.ResourceWithImportState = &CheckResource{}

func NewCheckResource() resource.Resource {
	return &CheckResource{}
}

// CheckResource defines the resource implementation.
type CheckResource struct {
	client     influxdb2.Client
	org        string
	serverURL  string
	authToken  string
	httpClient *http.Client
}

// CheckResourceModel describes the resource data model.
type CheckResourceModel struct {
	ID                    types.String     `tfsdk:"id"`
	Name                  types.String     `tfsdk:"name"`
	Org                   types.String     `tfsdk:"org"`
	Description           types.String     `tfsdk:"description"`
	Query                 types.String     `tfsdk:"query"`
	Status                types.String     `tfsdk:"status"`
	Every                 types.String     `tfsdk:"every"`
	Offset                types.String     `tfsdk:"offset"`
	StatusMessageTemplate types.String     `tfsdk:"status_message_template"`
	Type                  types.String     `tfsdk:"type"`
	Thresholds            []ThresholdModel `tfsdk:"thresholds"`
	CreatedAt             types.String     `tfsdk:"created_at"`
	UpdatedAt             types.String     `tfsdk:"updated_at"`
}

type ThresholdModel struct {
	Type      types.String  `tfsdk:"type"`
	Value     types.Float64 `tfsdk:"value"`
	Level     types.String  `tfsdk:"level"`
	AllValues types.Bool    `tfsdk:"all_values"`
}

// CheckAPI represents the structure used for InfluxDB Check API calls
type CheckAPI struct {
	ID                    *string          `json:"id,omitempty"`
	Name                  string           `json:"name"`
	OrgID                 string           `json:"orgID"`
	Description           *string          `json:"description,omitempty"`
	Query                 CheckQuery       `json:"query"`
	Status                string           `json:"status"`
	Every                 string           `json:"every"`
	Offset                string           `json:"offset"`
	StatusMessageTemplate *string          `json:"statusMessageTemplate,omitempty"`
	Thresholds            []CheckThreshold `json:"thresholds"`
	Type                  string           `json:"type"`
	CreatedAt             *string          `json:"createdAt,omitempty"`
	UpdatedAt             *string          `json:"updatedAt,omitempty"`
}

type CheckQuery struct {
	Text string `json:"text"`
}

type CheckThreshold struct {
	AllValues *bool   `json:"allValues,omitempty"`
	Level     string  `json:"level"`
	Value     float64 `json:"value"`
	Type      string  `json:"type"`
}

type CheckListResponse struct {
	Checks []CheckAPI `json:"checks"`
}

func (r *CheckResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_check"
}

func (r *CheckResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "InfluxDB check resource for monitoring and alerting",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Check ID",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Check name",
			},
			"org": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Organization name or ID. If not provided, uses the provider default.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Check description",
			},
			"query": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Flux query to execute for the check",
			},
			"status": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Check status (active or inactive). Defaults to active.",
			},
			"every": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Duration between check executions (e.g., '1m', '5m', '1h')",
			},
			"offset": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Optional offset for check execution timing. Defaults to '0s'.",
			},
			"status_message_template": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Template for status messages",
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Check type ('threshold' or 'deadman').",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Check creation timestamp",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Check last update timestamp",
			},
		},
		Blocks: map[string]schema.Block{
			"thresholds": schema.ListNestedBlock{
				MarkdownDescription: "Threshold definitions for the check",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Threshold comparison type (greater, lesser, equal, etc.)",
						},
						"value": schema.Float64Attribute{
							Required:            true,
							MarkdownDescription: "Threshold value to compare against",
						},
						"level": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Alert level (CRIT, WARN, INFO, OK)",
						},
						"all_values": schema.BoolAttribute{
							Optional:            true,
							Computed:            true,
							MarkdownDescription: "Whether to apply threshold to all values. Defaults to false.",
						},
					},
				},
			},
		},
	}
}

func (r *CheckResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = providerData.Client
	r.org = providerData.Org

	// Extract server URL and auth token for HTTP requests
	r.serverURL = providerData.URL
	r.authToken = providerData.Token
	r.httpClient = &http.Client{}
}

// makeHTTPRequest makes an HTTP request to the InfluxDB API
func (r *CheckResource) makeHTTPRequest(ctx context.Context, method, endpoint string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	url := fmt.Sprintf("%s%s", r.serverURL, endpoint)
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Token %s", r.authToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// setComputedFields sets computed fields from the check response
func (r *CheckResource) setComputedFields(data *CheckResourceModel, check *CheckAPI) {
	data.ID = types.StringValue(*check.ID)
	data.Name = types.StringValue(check.Name)

	if check.Description != nil {
		data.Description = types.StringValue(*check.Description)
	} else {
		data.Description = types.StringNull()
	}

	data.Query = types.StringValue(check.Query.Text)
	data.Status = types.StringValue(check.Status)
	data.Every = types.StringValue(check.Every)
	data.Offset = types.StringValue(check.Offset)
	data.Type = types.StringValue(check.Type)

	if check.StatusMessageTemplate != nil {
		data.StatusMessageTemplate = types.StringValue(*check.StatusMessageTemplate)
	} else {
		data.StatusMessageTemplate = types.StringNull()
	}

	// Set thresholds from API response
	data.Thresholds = make([]ThresholdModel, len(check.Thresholds))
	for i, threshold := range check.Thresholds {
		allValues := false
		if threshold.AllValues != nil {
			allValues = *threshold.AllValues
		}
		data.Thresholds[i] = ThresholdModel{
			Type:      types.StringValue(threshold.Type),
			Value:     types.Float64Value(threshold.Value),
			Level:     types.StringValue(threshold.Level),
			AllValues: types.BoolValue(allValues),
		}
	}

	// Set timestamps
	if check.CreatedAt != nil {
		data.CreatedAt = types.StringValue(*check.CreatedAt)
	} else {
		data.CreatedAt = types.StringNull()
	}
	if check.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(*check.UpdatedAt)
	} else {
		data.UpdatedAt = types.StringNull()
	}
}

func (r *CheckResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CheckResourceModel

	// Read Terraform plan data into the model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use provider org if not specified
	orgName := r.org
	if !data.Org.IsNull() {
		orgName = data.Org.ValueString()
	}

	// Resolve organization name to ID
	orgsAPI := r.client.OrganizationsAPI()
	org, err := orgsAPI.FindOrganizationByName(ctx, orgName)
	if err != nil {
		resp.Diagnostics.AddError("Create - Client Error", fmt.Sprintf("Unable to find organization '%s', got error: %s", orgName, err))
		return
	}

	// Prepare check payload
	checkPayload := CheckAPI{
		Name:  data.Name.ValueString(),
		OrgID: *org.Id,
		Query: CheckQuery{
			Text: data.Query.ValueString(),
		},
		Status:     "active",
		Every:      data.Every.ValueString(),
		Offset:     "0s",
		Type:       data.Type.ValueString(),
		Thresholds: make([]CheckThreshold, len(data.Thresholds)),
	}

	// Build thresholds array
	for i, threshold := range data.Thresholds {
		allValues := threshold.AllValues.ValueBool()
		checkPayload.Thresholds[i] = CheckThreshold{
			Type:      threshold.Type.ValueString(),
			Value:     threshold.Value.ValueFloat64(),
			Level:     threshold.Level.ValueString(),
			AllValues: &allValues,
		}
	}

	// Set optional fields
	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		checkPayload.Description = &desc
	}
	if !data.Status.IsNull() {
		checkPayload.Status = data.Status.ValueString()
	}
	if !data.Offset.IsNull() {
		checkPayload.Offset = data.Offset.ValueString()
	}
	if !data.Type.IsNull() {
		checkPayload.Type = data.Type.ValueString()
	}
	if !data.StatusMessageTemplate.IsNull() {
		template := data.StatusMessageTemplate.ValueString()
		checkPayload.StatusMessageTemplate = &template
	}

	// Debug: Print payload before sending
	payloadJSON, _ := json.MarshalIndent(checkPayload, "", "  ")
	resp.Diagnostics.AddWarning("Create - Payload Debug", fmt.Sprintf("Sending payload: %s", string(payloadJSON)))

	// Create check via HTTP API
	respBody, err := r.makeHTTPRequest(ctx, "POST", "/api/v2/checks", checkPayload)
	if err != nil {
		resp.Diagnostics.AddError("Create - HTTP Error", fmt.Sprintf("Unable to create check: %s", err))
		return
	}

	var createdCheck CheckAPI
	if err := json.Unmarshal(respBody, &createdCheck); err != nil {
		resp.Diagnostics.AddError("Create - Parse Error", fmt.Sprintf("Unable to parse check response: %s", err))
		return
	}

	// Save data into Terraform state
	data.Org = types.StringValue(orgName) // Keep the original organization name/identifier that was used in config
	r.setComputedFields(&data, &createdCheck)

	setDiags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(setDiags...)
}

func (r *CheckResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CheckResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get check by ID via HTTP API
	endpoint := fmt.Sprintf("/api/v2/checks/%s", data.ID.ValueString())
	respBody, err := r.makeHTTPRequest(ctx, "GET", endpoint, nil)
	if err != nil {
		resp.Diagnostics.AddError("Read - HTTP Error", fmt.Sprintf("Unable to read check: %s", err))
		return
	}

	var check CheckAPI
	if err := json.Unmarshal(respBody, &check); err != nil {
		resp.Diagnostics.AddError("Read - Parse Error", fmt.Sprintf("Unable to parse check response: %s", err))
		return
	}

	// Resolve organization ID to name for consistency
	orgsAPI := r.client.OrganizationsAPI()
	org, err := orgsAPI.FindOrganizationByID(ctx, check.OrgID)
	if err != nil {
		resp.Diagnostics.AddError("Read - Client Error", fmt.Sprintf("Unable to find organization with ID '%s', got error: %s", check.OrgID, err))
		return
	}
	data.Org = types.StringValue(org.Name)

	// Set computed fields
	r.setComputedFields(&data, &check)

	readSetDiags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(readSetDiags...)
}

func (r *CheckResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data CheckResourceModel

	// Read Terraform plan data into the model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Prepare check payload for update
	checkPayload := CheckAPI{
		ID:   data.ID.ValueStringPointer(),
		Name: data.Name.ValueString(),
		Query: CheckQuery{
			Text: data.Query.ValueString(),
		},
		Status:     data.Status.ValueString(),
		Every:      data.Every.ValueString(),
		Offset:     data.Offset.ValueString(),
		Type:       data.Type.ValueString(),
		Thresholds: make([]CheckThreshold, len(data.Thresholds)),
	}

	// Build thresholds array
	for i, threshold := range data.Thresholds {
		allValues := threshold.AllValues.ValueBool()
		checkPayload.Thresholds[i] = CheckThreshold{
			Type:      threshold.Type.ValueString(),
			Value:     threshold.Value.ValueFloat64(),
			Level:     threshold.Level.ValueString(),
			AllValues: &allValues,
		}
	}

	// Set optional fields
	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		checkPayload.Description = &desc
	}
	if !data.StatusMessageTemplate.IsNull() {
		template := data.StatusMessageTemplate.ValueString()
		checkPayload.StatusMessageTemplate = &template
	}

	// Debug: Print payload before sending
	payloadJSON, _ := json.MarshalIndent(checkPayload, "", "  ")
	resp.Diagnostics.AddWarning("Update - Payload Debug", fmt.Sprintf("Sending payload: %s", string(payloadJSON)))

	// Update check via HTTP API
	endpoint := fmt.Sprintf("/api/v2/checks/%s", data.ID.ValueString())
	respBody, err := r.makeHTTPRequest(ctx, "PATCH", endpoint, checkPayload)
	if err != nil {
		resp.Diagnostics.AddError("Update - HTTP Error", fmt.Sprintf("Unable to update check: %s", err))
		return
	}

	var updatedCheck CheckAPI
	if err := json.Unmarshal(respBody, &updatedCheck); err != nil {
		resp.Diagnostics.AddError("Update - Parse Error", fmt.Sprintf("Unable to parse check response: %s", err))
		return
	}

	// Update data from API response
	r.setComputedFields(&data, &updatedCheck)

	updateSetDiags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(updateSetDiags...)
}

func (r *CheckResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CheckResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete check via HTTP API
	endpoint := fmt.Sprintf("/api/v2/checks/%s", data.ID.ValueString())
	_, err := r.makeHTTPRequest(ctx, "DELETE", endpoint, nil)
	if err != nil {
		// Check if it's a 404 (not found) - this is okay for delete operations
		if strings.Contains(err.Error(), "404") {
			// Resource already deleted, consider this success
			return
		}
		resp.Diagnostics.AddError("Delete - HTTP Error", fmt.Sprintf("Unable to delete check: %s", err))
		return
	}
}

func (r *CheckResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import using check ID
	diags := resp.State.SetAttribute(ctx, path.Root("id"), req.ID)
	resp.Diagnostics.Append(diags...)
}
