---
subcategory: "helper"
layout: "kubernetes"
page_title: "Kubernetes: kubernetes_labels"
description: |-
  This resource allows Terraform to manage the labels for a resource that already exists
---

# kubernetes_labels

This resource allows Terraform to manage the labels for a resource that already exists.

~> **Note:** Existing labels must be included in the configuration for the resource, otherwise they will be destroyed when the resource is created. 

## Example Usage

```hcl
data "kubernetes_labels" "example" {
  api_version = "v1"
  kind        = "ConfigMap"
  metadata {
    name = "my-config"
  }
  labels = {
    "owner" = "myteam"
  }
}
```

## Argument Reference

The following arguments are supported:

* `api_version` - (Required) The apiVersion of the resource to be labelled.
* `kind` - (Required) The kind of the resource to be labelled.
* `metadata` - (Required) Standard metadata of the resource to be labelled. 
* `labels` - (Required) A map of labels to apply to the resource.

## Nested Blocks

### `metadata`

#### Arguments

* `name` - (Required) Name of the resource to be labelled.
* `namespace` - (Optional) Namespace of the resource to be labelled.


