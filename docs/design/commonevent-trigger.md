# Trigger Template for CommonEvent

## Abstract

We currently have two ways to trigger the diagnosis process, one is by monitoring kubernetes events and the other is by receiving Prometheus alert template events. The PagerDuty Common Event Format (PD-CEF) is a standardized alert format that allows you to view alert and incident data in a cleaner, more normalized way. They express CommonEvent concepts in a normalized, readable way.

The goal of this Task is to implement the trigger that supports CommonEvent, the diagnosis is automatically generated based on CommonEvent Template to trigger the diagnosis process.

## Design

The table below outlines the name and type of each PD-CEF field. See more details at [Common Event Format (PD-CEF)](https://support.pagerduty.com/docs/pd-cef).

|        Name        |                 Type                 |
| :----------------: | :----------------------------------: |
|    **Summary**     |                String                |
|     **Source**     |                String                |
|    **Severity**    | enum{Info, Warning, Error, Critical} |
|   **Timestamp**    |              Timestamp               |
|     **Class**      |                String                |
|   **Component**    |                String                |
|     **Group**      |                String                |
| **Custom Details** |                Object                |

The CommonEvent structure already contains all the above fields. See at [CommonEvent](https://github.com/kubediag/kubediag/blob/70e5cfd1c5b53b2f4f3be3b7a4d27bc1b2fa45d0/api/v1/commonevent_types.go#L24)

However, the fields Timestamp and CustomDetails fields respectively represent the time when the event was generated or created and the user-defined content. Summary is a high-level, text summary message of the event also. Therefore, the contents of three fields are not considered when defining the Trigger template for matching.

We define CommonEventTemplateRegexp as follows:

```go
// CommonEventTemplateRegexp is the regular expression for matching CommonEvent template.
// All regular expressions must be in the syntax accepted by RE2 and described at https://golang.org/s/re2syntax.
type CommonEventTemplateRegexp struct {
  // Source is the regular expression for matching "Source" of CommonEvent.
  // +optional
  Source string `json:"source,omitempty"`
  // Severity is the regular expression for matching "Severity" of CommonEvent.
  // +optional
  Severity string `json:"severity,omitempty"`
  // Class is the regular expression for matching "Class" of CommonEvent.
  // +optional
  Class string `json:"class,omitempty"`
  // Component is the regular expression for matching "Component" of CommonEvent.
  // +optional
  Component string `json:"component,omitempty"`
  // Group is the regular expression for matching "Group" of CommonEvent.
  // +optional
  Group string `json:"group,omitempty"`
}
```

CommonEvent resource will be created when CommonEvent happened but diagnosis is not sure created only if the CommonEvent is matched with one of triggers that predefined in cluster.

In addition, we also need to add a router for KubeDiag server in order to listen CommonEvent occur.

### Create diagnosis details

We should match CommonEvent and trigger template predefined before create diagnosis in truth. A possible CommonEvent trigger template may looks like following:

```yaml
apiVersion: diagnosis.kubediag.org/v1
kind: Trigger
metadata:
  name: cpu-high-level
spec:
  operationSet: cpu-high-debugger
  nodeName: example-node
  sourceTemplate:
    commonEventTemplate:
      regexp:
        source: 10.10.101.101
        severity: Error
        class: High CPU
        component: webPing
        group: www
```

We can't define any refer labels in CommonEvent trigger because CommonEvent structure does not contain any filed other than PD-CEF fields. So the nodeName of diagnosis will be created is determined in trigger resource, the nodeName field is necessary.

### Additional Details

1. CommonEventer structure is defined so that We can configure manually whether to enable this common events like alertmanager events and kubernetes events. Also a logger member to log message, a client for interacting with Kubernetes API servers and a cache for reading instances.
2. The state of the object needs to be updated if a CommonEvent object already exists in the cluster. The common event will also be triggered.

## Implement

To achieve this task, the following specific steps are required：

1. Firstly, define the CommonEventTemplate regular expression for matching CommonEvent object;
2. We should define a router for this event and correspondent handler;
3. Traverse all Triggers and match the regular expressions defined in the CommonEvent template. Create Diagnosis if match successfully.
