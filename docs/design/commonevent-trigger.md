## Abstract

We currently have two ways to trigger the diagnostic process, one is by monitoring kubernetes events, and the other is by receiving Prometheus alert templates events. The PagerDuty Common Event Format (PD-CEF) is a standardized alert format that allows you to view alert and incident data in a cleaner, more normalized way. They express common event concepts in a normalized, readable way.

The goal of this Task is to implement the trigger that supports Common Event, the diagnosis is automatically generated based on CommonEvent Template to trigger the diagnosis process.

## Design

The table below outlines the name and type of each PD-CEF field.

|        Name        |              Type              |
| :----------------: | :----------------------------: |
|    **Summary**     |             String             |
|     **Source**     |             String             |
|    **Severity**    | Info, Warning, Error, Critical |
|   **Timestamp**    |           Timestamp            |
|     **Class**      |             String             |
|   **Component**    |             String             |
|     **Group**      |             String             |
| **Custom Details** |             Object             |

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

However, the fields timestamp and CustomDetails respectively represent the time when the event was generated or created and the user-defined content. Summary is a high-level, text summary message of the event also.Therefore, the contents of three fields are not considered when defining the Trigger template for matching.

commonevent resource will be created when comment event happened but diagnosis is not sure created only if the commonevent is matched with one of triggers that predefined in cluster.

In addition, we also need to add a router for kubediag server in order to listen common event occur. 

#### Create diagnosis details



## Implement

To achieve this task, the following steps are required：

* Firstly, define the CommonEventTemplate regular experssion for matching CommonEvent;
* We should define a router for this event and correspondent handler;
* Traverse all Triggers and match the regular expressions defined in the CommonEvent template. Create Diagnosis if match successfully.

#### 1. Define CommonEventTemplate and Regexp for matching

The commonevent template refer to PrometheusAlertTemplate and KubernetesEventTemplate, the code looks like following:

#### 3. Handler

Now, we Implement the logic of this handler by accepting the request and create the common event object or update existing object status.

##### 3.1 Accept request body and deserialize to CommonEventFormat object

##### 3.2 Create CommonEvent object

The part is same as pagerdutyeventer, following is the code:

##### 3.3. Match CommonEvent object and tigger

Realize match function while an CommonEvent object is Created in cluster, the fields to match is defined in CommonEventTemplateRegexp. 



##### 3.4 Create diagnosis

We need to generate diagnosis if current CommonEvent object is matched to any trigger in cluster, the members of new diagnosis is determined by trigger Spec.







