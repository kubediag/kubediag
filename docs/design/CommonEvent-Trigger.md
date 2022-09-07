* ## Abstract

  We currently have two ways to trigger the diagnosis process, one is by monitoring kubernetes events and the other is by receiving Prometheus alert templates events. The PagerDuty Common Event Format (PD-CEF) is a standardized alert format that allows you to view alert and incident data in a cleaner, more normalized way. They express common event concepts in a normalized, readable way.

  The goal of this Task is to implement the trigger that supports Common Event, the diagnosis is automatically generated based on CommonEvent Template to trigger the diagnosis process.

  ## Design

  The table below outlines the name and type of each PD-CEF field. See more details at https://support.pagerduty.com/docs/pd-cef.

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

  Considering all the above fields, the following CommonEventFormat struct is designed:

  ```go
  type CommonEventFormat struct {
     Summary       string            `json:"summary,omitempty"`
     Source        string            `json:"source,omitempty"`
     Severity      string            `json:"severity,omitempty"`
     Timestamp     string            `json:"timestamp,omitempty"`
     Class         string            `json:"class,omitempty"`
     Component     string            `json:"component,omitempty"`
     Group         string            `json:"group,omitempty"`
     CustomDetails map[string]string `json:"custom_details,omitempty"`
  }
  ```

  However, the fields Timestamp and CustomDetails respectively represent the time when the event was generated or created and the user-defined content. Summary is a high-level, text summary message of the event also.Therefore, the contents of three fields are not considered when defining the Trigger template for matching.

  We define CommonEventTemplateRegexp as follows:

  ```go
  // CommonEventTemplateRegexp is the regular expression for matching common event template.
  // All regular expressions must be in the syntax accepted by RE2 and described at https://golang.org/s/re2syntax.
  type CommonEventTemplateRegexp struct {
     // Source is the regular expression for matching "Source" of common event.
     // +optional
     Source string `json:"source,omitempty"`
     // Severity is the regular expression for matching "Severity" of common event.
     // +optional
     Severity string `json:"severity,omitempty"`
     // Class is the regular expression for matching "Class" of common event.
     // +optional
     Class string `json:"class,omitempty"`
     // Component is the regular expression for matching "Component" of common event.
     // +optional
     Component string `json:"component,omitempty"`
     // Group is the regular expression for matching "Group" of common event.
     // +optional
     Group string `json:"group,omitempty"`
  }
  ```

  commonevent resource will be created when comment event happened but diagnosis is not sure created only if the commonevent is matched with one of triggers that predefined in cluster.

  In addition, we also need to add a router for kubediag server in order to listen common event occur.

  #### Create diagnosis details

  We should match commonevent and trigger template predefined before create diagnosis in truth. A possible commonevent trigger template may looks like following:

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

  It's worth noting that we can't define any refer labels in common event trigger beacuse CommonEventFormat struct does not contain any filed other than PD-CEF fileds. So the nodeName of diagnosis will be created is determined in trigger resourece, the nodeName is necessary.

  #### Additional details

  1. CommonEventer struct is defined so taht We can configure manually whether to enable this common events like alertmanager events and kubernetes events. Also a logger member to log message, a client for interacting with Kubernetes API servers and a cache for reading instances.
  2. The state of the object needs to be updated If a common event object already exists in the cluster. The common event will also be triggered.

  ## Implement

  To achieve this task, the following specific steps are required：

  * Firstly, define the CommonEventTemplate regular experssion for matching CommonEvent object;
  * We should define a router for this event and correspondent handler;
  * Traverse all Triggers and match the regular expressions defined in the CommonEvent template. Create Diagnosis if match successfully.