package azurerm

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/sql/mgmt/2015-05-01-preview/sql"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/terraform-providers/terraform-provider-azurerm/azurerm/utils"
)

func resourceArmSqlElasticPool() *schema.Resource {
	return &schema.Resource{
		Create: resourceArmSqlElasticPoolCreate,
		Read:   resourceArmSqlElasticPoolRead,
		Update: resourceArmSqlElasticPoolCreate,
		Delete: resourceArmSqlElasticPoolDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"location": locationSchema(),

			"resource_group_name": resourceGroupNameSchema(),

			"server_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"edition": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validateSqlElasticPoolEdition(),
			},

			"dtu": {
				Type:     schema.TypeInt,
				Required: true,
			},

			"db_dtu_min": {
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
			},

			"db_dtu_max": {
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
			},

			"pool_size": {
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
			},

			"creation_date": {
				Type:     schema.TypeString,
				Computed: true,
			},

			"tags": tagsSchema(),
		},
	}
}

func resourceArmSqlElasticPoolCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).sqlElasticPoolsClient

	log.Printf("[INFO] preparing arguments for SQL ElasticPool creation.")

	name := d.Get("name").(string)
	serverName := d.Get("server_name").(string)
	location := d.Get("location").(string)
	resGroup := d.Get("resource_group_name").(string)
	tags := d.Get("tags").(map[string]interface{})

	elasticPool := sql.ElasticPool{
		Name:                  &name,
		Location:              &location,
		ElasticPoolProperties: getArmSqlElasticPoolProperties(d),
		Tags: expandTags(tags),
	}

	future, err := client.CreateOrUpdate(context.TODO(), resGroup, serverName, name, elasticPool)
	if err != nil {
		return err
	}

	err = future.WaitForCompletion(context.TODO(), client.Client)
	if err != nil {
		return err
	}

	read, err := client.Get(context.TODO(), resGroup, serverName, name)
	if err != nil {
		return err
	}
	if read.ID == nil {
		return fmt.Errorf("Cannot read SQL ElasticPool %q (resource group %q) ID", name, resGroup)
	}

	d.SetId(*read.ID)

	return resourceArmSqlElasticPoolRead(d, meta)
}

func resourceArmSqlElasticPoolRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient).sqlElasticPoolsClient

	resGroup, serverName, name, err := parseArmSqlElasticPoolId(d.Id())
	if err != nil {
		return err
	}

	resp, err := client.Get(context.TODO(), resGroup, serverName, name)
	if err != nil {
		if utils.ResponseWasNotFound(resp.Response) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("Error making Read request on Sql Elastic Pool %s: %s", name, err)
	}

	d.Set("name", resp.Name)
	d.Set("resource_group_name", resGroup)
	d.Set("location", azureRMNormalizeLocation(*resp.Location))
	d.Set("server_name", serverName)

	if elasticPool := resp.ElasticPoolProperties; elasticPool != nil {
		d.Set("edition", string(elasticPool.Edition))
		d.Set("dtu", int(*elasticPool.Dtu))
		d.Set("db_dtu_min", int(*elasticPool.DatabaseDtuMin))
		d.Set("db_dtu_max", int(*elasticPool.DatabaseDtuMax))
		d.Set("pool_size", int(*elasticPool.StorageMB))

		if date := elasticPool.CreationDate; date != nil {
			d.Set("creation_date", date.Format(time.RFC3339))
		}
	}

	flattenAndSetTags(d, resp.Tags)

	return nil
}

func resourceArmSqlElasticPoolDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*ArmClient)
	elasticPoolsClient := client.sqlElasticPoolsClient

	resGroup, serverName, name, err := parseArmSqlElasticPoolId(d.Id())
	if err != nil {
		return err
	}

	_, err = elasticPoolsClient.Delete(context.TODO(), resGroup, serverName, name)

	return err
}

func getArmSqlElasticPoolProperties(d *schema.ResourceData) *sql.ElasticPoolProperties {
	edition := sql.ElasticPoolEdition(d.Get("edition").(string))
	dtu := int32(d.Get("dtu").(int))

	props := &sql.ElasticPoolProperties{
		Edition: edition,
		Dtu:     &dtu,
	}

	if databaseDtuMin, ok := d.GetOk("db_dtu_min"); ok {
		databaseDtuMin := int32(databaseDtuMin.(int))
		props.DatabaseDtuMin = &databaseDtuMin
	}

	if databaseDtuMax, ok := d.GetOk("db_dtu_max"); ok {
		databaseDtuMax := int32(databaseDtuMax.(int))
		props.DatabaseDtuMax = &databaseDtuMax
	}

	if poolSize, ok := d.GetOk("pool_size"); ok {
		poolSize := int32(poolSize.(int))
		props.StorageMB = &poolSize
	}

	return props
}

func parseArmSqlElasticPoolId(sqlElasticPoolId string) (string, string, string, error) {
	id, err := parseAzureResourceID(sqlElasticPoolId)
	if err != nil {
		return "", "", "", fmt.Errorf("[ERROR] Unable to parse SQL ElasticPool ID %q: %+v", sqlElasticPoolId, err)
	}

	return id.ResourceGroup, id.Path["servers"], id.Path["elasticPools"], nil
}

func validateSqlElasticPoolEdition() schema.SchemaValidateFunc {
	return validation.StringInSlice([]string{
		string(sql.ElasticPoolEditionBasic),
		string(sql.ElasticPoolEditionStandard),
		string(sql.ElasticPoolEditionPremium),
	}, false)
}
