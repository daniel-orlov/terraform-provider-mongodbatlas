package mongodbatlas

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	matlas "go.mongodb.org/atlas/mongodbatlas"
)

const (
	errorPrivateEndpointRegionalModeRead    = "error reading MongoDB Private Endpoints Connection(%s): %s"
	errorPrivateEndpointRegionalModeSetting = "error setting `%s` for MongoDB Private Endpoints Connection(%s): %s"
)

func resourceMongoDBAtlasPrivateEndpointRegionalMode() *schema.Resource {
	return &schema.Resource{
		ReadContext:   resourceMongoDBAtlasPrivateEndpointRegionalModeRead,
		UpdateContext: resourceMongoDBAtlasPrivateEndpointRegionalModeUpdate,
		Importer: &schema.ResourceImporter{
			StateContext: resourceMongoDBAtlasPrivateLinkEndpointImportState,
		},
		Schema: map[string]*schema.Schema{
			"project_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"enabled": {
				Type:     schema.TypeBool,
				Required: true,
			},
		},
	}
}

func resourceMongoDBAtlasPrivateEndpointRegionalModeRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*MongoDBClient).Atlas

	ids := decodeStateID(d.Id())
	projectID := ids["project_id"]

	setting, resp, err := conn.PrivateEndpoints.GetRegionalizedPrivateEndpointSetting(context.Background(), projectID)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			d.SetId("")
			return nil
		}

		return diag.FromErr(fmt.Errorf(errorPrivateEndpointRegionalModeRead, projectID, err))
	}

	if err := d.Set("enabled", setting.Enabled); err != nil {
		return diag.FromErr(fmt.Errorf(errorPrivateLinkEndpointsSetting, "enabled", projectID, err))
	}

	return nil
}

func resourceMongoDBAtlasPrivateEndpointRegionalModeUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	conn := meta.(*MongoDBClient).Atlas

	ids := decodeStateID(d.Id())
	projectID := ids["project_id"]
	enabled := d.Get("enabled").(bool)

	_, resp, err := conn.PrivateEndpoints.UpdateRegionalizedPrivateEndpointSetting(ctx, projectID, enabled)
	if err != nil {
		if resp.Response.StatusCode == 404 {
			return nil
		}

		return diag.FromErr(fmt.Errorf(errorPrivateLinkEndpointsDelete, projectID, err))
	}

	log.Println("[INFO] Waiting for MongoDB Private Endpoints Connection to be destroyed")

	stateConf := &resource.StateChangeConf{
		Pending:    []string{"DELETING"},
		Target:     []string{"DELETED", "FAILED"},
		Refresh:    resourcePrivateEndpointRegionalModeRefreshFunc(ctx, conn, projectID),
		Timeout:    1 * time.Hour,
		MinTimeout: 5 * time.Second,
		Delay:      3 * time.Second,
	}
	// Wait, catching any errors
	_, err = stateConf.WaitForStateContext(ctx)
	if err != nil {
		return diag.FromErr(fmt.Errorf(errorPrivateLinkEndpointsDelete, projectID, err))
	}

	return nil
}

func resourceMongoDBAtlasPrivateEndpointRegionalModeImportState(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	conn := meta.(*MongoDBClient).Atlas
	projectID := d.Id()

	setting, _, err := conn.PrivateEndpoints.GetRegionalizedPrivateEndpointSetting(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("couldn't import regional mode for project %s error: %s", projectID, err)
	}

	if err := d.Set("project_id", projectID); err != nil {
		log.Printf(errorPrivateLinkEndpointsSetting, "project_id", projectID, err)
	}

	if err := d.Set("enabled", setting.Enabled); err != nil {
		log.Printf(errorPrivateLinkEndpointsSetting, "enabled", projectID, err)
	}

	d.SetId(encodeStateID(map[string]string{
		"project_id": projectID,
	}))

	return []*schema.ResourceData{d}, nil
}

func resourcePrivateEndpointRegionalModeRefreshFunc(ctx context.Context, client *matlas.Client, projectID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		p, resp, err := client.PrivateEndpoints.GetRegionalizedPrivateEndpointSetting(ctx, projectID)
		if err != nil {
			if resp.Response.StatusCode == 404 {
				return "", "DELETED", nil
			}

			return nil, "REJECTED", err
		}

		return p, "Ok", nil
	}
}
