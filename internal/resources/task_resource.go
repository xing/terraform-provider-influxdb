package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"github.com/new-work/influxdb-provider/internal/common"
)

// fluxNormalizationModifier normalizes flux queries for comparison
type fluxNormalizationModifier struct{}

func (m fluxNormalizationModifier) Description(ctx context.Context) string {
	return "Normalizes flux whitespace for comparison"
}

func (m fluxNormalizationModifier) MarkdownDescription(ctx context.Context) string {
	return "Normalizes flux whitespace for comparison"
}

// normalizeFluxForComparison removes all leading/trailing whitespace and normalizes line breaks
func normalizeFluxForComparison(flux string) string {
	lines := strings.Split(flux, "\n")
	var normalizedLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			normalizedLines = append(normalizedLines, trimmed)
		}
	}

	return strings.Join(normalizedLines, "\n")
}

func (m fluxNormalizationModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// If either config or state is null/unknown, don't modify
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}

	// Normalize both values for comparison
	normalizedConfig := normalizeFluxForComparison(req.ConfigValue.ValueString())
	normalizedState := normalizeFluxForComparison(req.StateValue.ValueString())

	// If normalized values are equal, keep the state value to prevent drift
	if normalizedConfig == normalizedState {
		resp.PlanValue = req.StateValue
	}
}

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TaskResource{}
var _ resource.ResourceWithImportState = &TaskResource{}

func NewTaskResource() resource.Resource {
	return &TaskResource{}
}

// TaskResource defines the resource implementation.
type TaskResource struct {
	client influxdb2.Client
	org    string
}

// TaskResourceModel describes the resource data model.
type TaskResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Org         types.String `tfsdk:"org"`
	Description types.String `tfsdk:"description"`
	Flux        types.String `tfsdk:"flux"`
	Status      types.String `tfsdk:"status"`
	Every       types.String `tfsdk:"every"`
	Cron        types.String `tfsdk:"cron"`
	Offset      types.String `tfsdk:"offset"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func (r *TaskResource) stripOptionTaskLine(flux string) string {
	// Find and remove the option task pattern at the beginning
	result := flux
	if strings.Contains(flux, "option task = {") {
		// Find the end of the option task declaration
		start := strings.Index(flux, "option task = {")
		if start != -1 {
			// Find the matching closing brace
			braceCount := 0
			end := start
			for i := start; i < len(flux); i++ {
				if flux[i] == '{' {
					braceCount++
				} else if flux[i] == '}' {
					braceCount--
					if braceCount == 0 {
						end = i + 1
						break
					}
				}
			}

			// Remove the option task part and any following whitespace
			result = strings.TrimSpace(flux[end:])
		}
	}

	// Just return the result after stripping the option task - preserve original formatting
	return result
}

func (r *TaskResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_task"
}

func (r *TaskResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "InfluxDB task resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Task ID",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Task name",
			},
			"org": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Organization name or ID. If not provided, uses the provider default.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Task description",
			},
			"flux": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Flux script to execute",
				PlanModifiers: []planmodifier.String{
					fluxNormalizationModifier{},
				},
			},
			"status": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Task status (active or inactive). Defaults to active.",
			},
			"every": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Duration-based schedule (e.g., '1h', '30m'). Either 'every' or 'cron' must be specified.",
			},
			"cron": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Cron-based schedule (e.g., '0 */1 * * *'). Either 'every' or 'cron' must be specified.",
			},
			"offset": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional time offset for scheduling",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Task creation timestamp",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Task last update timestamp",
			},
		},
	}
}

func (r *TaskResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*common.ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = providerData.Client
	r.org = providerData.Org
}

// validateScheduling ensures either 'every' or 'cron' is specified, but not both
func (r *TaskResource) validateScheduling(data *TaskResourceModel, diagnostics *diag.Diagnostics) bool {
	hasEvery := !data.Every.IsNull() && data.Every.ValueString() != ""
	hasCron := !data.Cron.IsNull() && data.Cron.ValueString() != ""

	if !hasEvery && !hasCron {
		diagnostics.AddError("Validation Error", "Either 'every' or 'cron' must be specified for task scheduling")
		return false
	}

	if hasEvery && hasCron {
		diagnostics.AddError("Validation Error", "Cannot specify both 'every' and 'cron' scheduling options")
		return false
	}

	return true
}

// setComputedFields sets computed fields from the task response
func (r *TaskResource) setComputedFields(data *TaskResourceModel, task *domain.Task) {
	data.ID = types.StringValue(task.Id)
	data.Name = types.StringValue(task.Name)

	if task.Description != nil {
		data.Description = types.StringValue(*task.Description)
	} else {
		data.Description = types.StringNull()
	}

	// Set status with default
	if task.Status != nil {
		data.Status = types.StringValue(string(*task.Status))
	} else {
		data.Status = types.StringValue("active")
	}

	// Set scheduling fields
	if task.Every != nil {
		data.Every = types.StringValue(*task.Every)
	} else {
		data.Every = types.StringNull()
	}
	if task.Cron != nil {
		data.Cron = types.StringValue(*task.Cron)
	} else {
		data.Cron = types.StringNull()
	}
	if task.Offset != nil {
		data.Offset = types.StringValue(*task.Offset)
	} else {
		data.Offset = types.StringNull()
	}

	// Set timestamps - only set CreatedAt during Create
	if task.CreatedAt != nil {
		data.CreatedAt = types.StringValue(task.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		data.CreatedAt = types.StringNull()
	}
	// Note: We don't set UpdatedAt here - it should only be set during actual Update operations
	// This prevents Terraform from thinking it will change on subsequent applies
}

func (r *TaskResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TaskResourceModel

	// Read Terraform plan data into the model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate scheduling
	if !r.validateScheduling(&data, &resp.Diagnostics) {
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

	// Prepare task
	task := &domain.Task{
		Name:  data.Name.ValueString(),
		OrgID: *org.Id,
		Flux:  r.stripOptionTaskLine(data.Flux.ValueString()),
	}

	// Set optional description
	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		task.Description = &desc
	}

	// Set status (default to active)
	status := domain.TaskStatusTypeActive
	if !data.Status.IsNull() {
		status = domain.TaskStatusType(data.Status.ValueString())
	}
	task.Status = &status

	// Set scheduling
	if !data.Every.IsNull() {
		every := data.Every.ValueString()
		task.Every = &every
	}
	if !data.Cron.IsNull() {
		cron := data.Cron.ValueString()
		task.Cron = &cron
	}
	if !data.Offset.IsNull() {
		offset := data.Offset.ValueString()
		task.Offset = &offset
	}

	// Create task
	tasksAPI := r.client.TasksAPI()
	createdTask, err := tasksAPI.CreateTask(ctx, task)
	if err != nil {
		resp.Diagnostics.AddError("Create - Client Error", fmt.Sprintf("Unable to create task, got error: %s", err))
		return
	}

	// Save data into Terraform state
	data.Org = types.StringValue(orgName) // Keep the original organization name/identifier that was used in config
	r.setComputedFields(&data, createdTask)

	// Ensure updated_at is never null - if InfluxDB doesn't provide it, use created_at
	if data.UpdatedAt.IsNull() || data.UpdatedAt.IsUnknown() {
		data.UpdatedAt = data.CreatedAt
	}

	setDiags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(setDiags...)
}

func (r *TaskResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TaskResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get task by ID
	tasksAPI := r.client.TasksAPI()
	task, err := tasksAPI.GetTaskByID(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read - Client Error", fmt.Sprintf("Unable to read task, got error: %s", err))
		return
	}

	// Preserve stable computed fields from existing state (these should never change after creation)
	// Keep ID, CreatedAt, Org, UpdatedAt exactly as they are to prevent unnecessary drift
	// UpdatedAt should only change when we actually modify the task, not on reads
	// (data.ID, data.CreatedAt, data.Org, data.UpdatedAt already have correct values from req.State.Get)

	// Update fields that can actually change externally
	data.Name = types.StringValue(task.Name)

	if task.Description != nil {
		data.Description = types.StringValue(*task.Description)
	} else {
		data.Description = types.StringNull()
	}

	// Strip InfluxDB's automatic option task line from flux
	data.Flux = types.StringValue(r.stripOptionTaskLine(task.Flux))

	if task.Status != nil {
		data.Status = types.StringValue(string(*task.Status))
	} else {
		data.Status = types.StringValue("active")
	}

	if task.Cron != nil {
		data.Cron = types.StringValue(*task.Cron)
	} else {
		data.Cron = types.StringNull()
	}

	if task.Every != nil {
		data.Every = types.StringValue(*task.Every)
	} else {
		data.Every = types.StringNull()
	}

	if task.Offset != nil {
		data.Offset = types.StringValue(*task.Offset)
	} else {
		data.Offset = types.StringNull()
	}

	// Note: We don't update UpdatedAt in Read method - preserve existing state value
	// This prevents unnecessary drift when InfluxDB hasn't actually updated the timestamp	// Always set state - let Terraform framework handle change detection
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TaskResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TaskResourceModel
	var state TaskResourceModel

	// Read Terraform plan data (new values) into the model
	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read current state data (to get the ID and other computed fields)
	stateDiags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(stateDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Use stable computed fields from state (these are not in plan but should be preserved)
	data.ID = state.ID
	data.CreatedAt = state.CreatedAt
	data.Org = state.Org // Preserve org from state to prevent inconsistent result

	// Validate scheduling
	if !r.validateScheduling(&data, &resp.Diagnostics) {
		return
	}

	// Get the current task to retrieve OrgID
	tasksAPI := r.client.TasksAPI()

	taskID := data.ID.ValueString()

	currentTask, err := tasksAPI.GetTaskByID(ctx, taskID)
	if err != nil {
		resp.Diagnostics.AddError("Update - Client Error", fmt.Sprintf("Unable to read current task, got error: %s", err))
		return
	}

	// For the flux field, we need to preserve InfluxDB's option task structure
	// but update the actual query content. We'll use the current task's flux
	// but replace the stripped content with our new content
	var updatedFlux string
	if strings.Contains(currentTask.Flux, "option task = {") {
		// Find where the actual flux query starts (after the option task line)
		start := strings.Index(currentTask.Flux, "option task = {")
		braceCount := 0
		end := start
		for i := start; i < len(currentTask.Flux); i++ {
			if currentTask.Flux[i] == '{' {
				braceCount++
			} else if currentTask.Flux[i] == '}' {
				braceCount--
				if braceCount == 0 {
					end = i + 1
					break
				}
			}
		}

		// Replace the content after the option task with our new flux (normalized)
		optionPart := currentTask.Flux[:end]
		normalizedFlux := r.stripOptionTaskLine(data.Flux.ValueString())
		updatedFlux = optionPart + " " + normalizedFlux
	} else {
		// No option task exists, just use normalized flux
		updatedFlux = r.stripOptionTaskLine(data.Flux.ValueString())
	}

	// Prepare task for update with required OrgID
	task := &domain.Task{
		Id:    taskID,
		Name:  data.Name.ValueString(),
		Flux:  updatedFlux,
		OrgID: currentTask.OrgID, // Include OrgID from current task
	} // Set optional description
	if !data.Description.IsNull() {
		desc := data.Description.ValueString()
		task.Description = &desc
	}

	// Set status
	if !data.Status.IsNull() {
		status := domain.TaskStatusType(data.Status.ValueString())
		task.Status = &status
	}

	// Set scheduling
	if !data.Every.IsNull() {
		every := data.Every.ValueString()
		task.Every = &every
	}
	if !data.Cron.IsNull() {
		cron := data.Cron.ValueString()
		task.Cron = &cron
	}
	if !data.Offset.IsNull() {
		offset := data.Offset.ValueString()
		task.Offset = &offset
	}

	// Update task - first let's try with a more complete task object
	// Copy all fields from currentTask and then override with new values
	task.CreatedAt = currentTask.CreatedAt
	task.UpdatedAt = currentTask.UpdatedAt

	updatedTask, err := tasksAPI.UpdateTask(ctx, task)
	if err != nil {
		resp.Diagnostics.AddError("Update - Client Error", fmt.Sprintf("Unable to update task, got error: %s", err))
		return
	}

	// Note: We don't update data.Org here since it's preserved from state above
	// and has UseStateForUnknown() plan modifier to prevent drift

	// Update timestamp from API response since this is an actual update operation
	if updatedTask.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(updatedTask.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
	}

	updateSetDiags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(updateSetDiags...)
}

func (r *TaskResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TaskResourceModel

	// Read Terraform prior state data into the model
	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete task
	tasksAPI := r.client.TasksAPI()
	task := &domain.Task{Id: data.ID.ValueString()}
	err := tasksAPI.DeleteTask(ctx, task)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete task, got error: %s", err))
		return
	}
}

func (r *TaskResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import using task ID
	diags := resp.State.SetAttribute(ctx, path.Root("id"), req.ID)
	resp.Diagnostics.Append(diags...)
}
