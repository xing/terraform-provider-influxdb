package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/domain"
)

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
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Task name",
			},
			"org": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Organization name or ID. If not provided, uses the provider default.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Task description",
			},
			"flux": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Flux script to execute",
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

	// Set timestamps
	if task.CreatedAt != nil {
		data.CreatedAt = types.StringValue(task.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		data.CreatedAt = types.StringNull()
	}
	if task.UpdatedAt != nil {
		data.UpdatedAt = types.StringValue(task.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		data.UpdatedAt = types.StringNull()
	}
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
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find organization '%s', got error: %s", orgName, err))
		return
	}

	// Prepare task
	task := &domain.Task{
		Name:  data.Name.ValueString(),
		OrgID: *org.Id,
		Flux:  data.Flux.ValueString(),
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
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create task, got error: %s", err))
		return
	}

	// Save data into Terraform state
	data.Org = types.StringValue(orgName) // Keep the original organization name/identifier that was used in config
	r.setComputedFields(&data, createdTask)

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
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read task, got error: %s", err))
		return
	}

	// Update data from API response
	data.Flux = types.StringValue(task.Flux)

	// Resolve organization ID to name for consistency
	orgsAPI := r.client.OrganizationsAPI()
	org, err := orgsAPI.FindOrganizationByID(ctx, task.OrgID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find organization with ID '%s', got error: %s", task.OrgID, err))
		return
	}
	data.Org = types.StringValue(org.Name)

	// Set computed fields
	r.setComputedFields(&data, task)

	readSetDiags := resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(readSetDiags...)
}

func (r *TaskResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
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

	// Prepare task for update
	task := &domain.Task{
		Id:   data.ID.ValueString(),
		Name: data.Name.ValueString(),
		Flux: data.Flux.ValueString(),
	}

	// Set optional description
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

	// Update task
	tasksAPI := r.client.TasksAPI()
	updatedTask, err := tasksAPI.UpdateTask(ctx, task)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update task, got error: %s", err))
		return
	}

	// Update data from API response
	data.Flux = types.StringValue(updatedTask.Flux)
	r.setComputedFields(&data, updatedTask)

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
