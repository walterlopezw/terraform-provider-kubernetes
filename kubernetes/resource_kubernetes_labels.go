package kubernetes

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
			"metadata": {
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
			"force": {
				Type:        schema.TypeBool,
				Description: "Force overwriting labels that were created or edited outside of Terraform.",
				Optional:    true,
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

	configuredLabels := d.Get("labels").(map[string]interface{})

	// strip out the labels not managed by Terraform
	managedLabels, err := getManagedLabels(res.GetManagedFields(), defaultFieldManagerName)
	if err != nil {
		return diag.FromErr(err)
	}
	labels := res.GetLabels()
	for k := range labels {
		_, managed := managedLabels["f:"+k]
		_, configured := configuredLabels[k]
		if !managed && !configured {
			delete(labels, k)
		}
	}

	d.Set("labels", labels)
	return nil
}

// getManagedLabels reads the field manager metadata to discover which fields we're managing
func getManagedLabels(managedFields []v1.ManagedFieldsEntry, manager string) (map[string]interface{}, error) {
	var labels map[string]interface{}
	for _, m := range managedFields {
		if m.Manager != manager {
			continue
		}
		var mm map[string]interface{}
		err := json.Unmarshal(m.FieldsV1.Raw, &mm)
		if err != nil {
			return nil, err
		}
		metadata := mm["f:metadata"].(map[string]interface{})
		if l, ok := metadata["f:labels"].(map[string]interface{}); ok {
			labels = l
		}
	}
	return labels, nil
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
	namespacedResource := mapping.Scope.Name() == meta.RESTScopeNameNamespace
	if namespacedResource {
		if namespace == "" {
			namespace = "default"
		}
		r = conn.Resource(mapping.Resource).Namespace(namespace)
	} else {
		r = conn.Resource(mapping.Resource)
	}

	// craft the patch to update the labels
	labels := d.Get("labels")
	if d.Id() == "" {
		// if we're deleting then just we just patch
		// with an empty labels map
		labels = map[string]interface{}{}
	}
	patchmeta := map[string]interface{}{
		"name":   name,
		"labels": labels,
	}
	if namespacedResource {
		patchmeta["namespace"] = namespace
	}
	patchobj := map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   patchmeta,
	}
	patch := unstructured.Unstructured{}
	patch.Object = patchobj
	patchbytes, err := patch.MarshalJSON()
	if err != nil {
		return diag.FromErr(err)
	}
	// apply the patch
	_, err = r.Patch(ctx,
		name,
		types.ApplyPatchType,
		patchbytes,
		v1.PatchOptions{
			FieldManager: defaultFieldManagerName,
			Force:        ptrToBool(d.Get("force").(bool)),
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
