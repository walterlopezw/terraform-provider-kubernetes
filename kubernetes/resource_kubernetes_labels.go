package kubernetes

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

func resourceKubernetesLabels() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesLabelsCreate,
		ReadContext:   resourceKubernetesLabelsRead,
		UpdateContext: resourceKubernetesLabelsUpdate,
		DeleteContext: resourceKubernetesLabelsDelete,
		Schema: map[string]*schema.Schema{
			"api_version": {
				Type:        schema.TypeString,
				Description: "The apiVersion of the resource to label.",
				Required:    true,
				ForceNew:    true,
			},
			"kind": {
				Type:        schema.TypeString,
				Description: "The kind of the resource to label.",
				Required:    true,
				ForceNew:    true,
			},
			"metadata": &schema.Schema{
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Description: "The name of the resource.",
							Required:    true,
							ForceNew:    true,
						},
						"namespace": {
							Type:        schema.TypeString,
							Description: "The namespace of the resource.",
							Optional:    true,
							ForceNew:    true,
						},
					},
				},
			},
			"labels": {
				Type:        schema.TypeMap,
				Description: "A map of labels to apply to the resource.",
				Required:    true,
			},
		},
	}
}

func resourceKubernetesLabelsCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	metadata := expandMetadata(d.Get("metadata").([]interface{}))
	d.SetId(buildIdWithVersionKind(metadata,
		d.Get("api_version").(string),
		d.Get("kind").(string)))
	return resourceKubernetesLabelsUpdate(ctx, d, m)
}

func resourceKubernetesLabelsRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn, err := m.(KubeClientsets).DynamicClient()
	if err != nil {
		return diag.FromErr(err)
	}

	apiVersion := d.Get("api_version").(string)
	kind := d.Get("kind").(string)
	metadata := expandMetadata(d.Get("metadata").([]interface{}))
	name := metadata.GetName()
	namespace := metadata.GetNamespace()

	// figure out which resource client to use
	dc, err := m.(KubeClientsets).DiscoveryClient()
	if err != nil {
		return diag.FromErr(err)
	}
	agr, err := restmapper.GetAPIGroupResources(dc)
	restMapper := restmapper.NewDiscoveryRESTMapper(agr)
	gv, err := k8sschema.ParseGroupVersion(apiVersion)
	if err != nil {
		return diag.FromErr(err)

	}
	mapping, err := restMapper.RESTMapping(gv.WithKind(kind).GroupKind(), gv.Version)
	if err != nil {
		return diag.FromErr(err)
	}

	// determine if the resource is namespaced or not
	var r dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		if namespace == "" {
			namespace = "default"
		}
		r = conn.Resource(mapping.Resource).Namespace(namespace)
	} else {
		r = conn.Resource(mapping.Resource)
	}

	// get the resource labels
	res, err := r.Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("labels", res.GetLabels())
	return nil
}

func resourceKubernetesLabelsUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	conn, err := m.(KubeClientsets).DynamicClient()
	if err != nil {
		return diag.FromErr(err)
	}

	apiVersion := d.Get("api_version").(string)
	kind := d.Get("kind").(string)
	metadata := expandMetadata(d.Get("metadata").([]interface{}))
	name := metadata.GetName()
	namespace := metadata.GetNamespace()

	// figure out which resource client to use
	dc, err := m.(KubeClientsets).DiscoveryClient()
	if err != nil {
		return diag.FromErr(err)
	}
	agr, err := restmapper.GetAPIGroupResources(dc)
	restMapper := restmapper.NewDiscoveryRESTMapper(agr)
	gv, err := k8sschema.ParseGroupVersion(apiVersion)
	if err != nil {
		return diag.FromErr(err)

	}
	mapping, err := restMapper.RESTMapping(gv.WithKind(kind).GroupKind(), gv.Version)
	if err != nil {
		return diag.FromErr(err)
	}

	// determine if the resource is namespaced or not
	var r dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		if namespace == "" {
			namespace = "default"
		}
		r = conn.Resource(mapping.Resource).Namespace(namespace)
	} else {
		r = conn.Resource(mapping.Resource)
	}

	old, new := d.GetChange("labels")
	if d.Id() == "" {
		// if the ID is empty we're deleting the resource
		new = map[string]interface{}{}
	}
	patch := diffStringMap("/metadata/labels/",
		old.(map[string]interface{}),
		new.(map[string]interface{}))
	patchbytes, err := patch.MarshalJSON()
	if err != nil {
		return diag.FromErr(err)
	}

	// apply the patch
	_, err = r.Patch(ctx,
		name,
		types.JSONPatchType,
		patchbytes,
		v1.PatchOptions{
			FieldManager: defaultFieldManagerName,
		},
	)
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceKubernetesLabelsRead(ctx, d, m)
}

func resourceKubernetesLabelsDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	d.SetId("")
	return resourceKubernetesLabelsUpdate(ctx, d, m)
}
