## Trigger for CommonEvent

The goal of this Task is to implement the trigger that supports CommonEvent, diagnosis is automatically generated based on CommonEvent Template to trigger the diagnosis process.

## Implementation

To achieve this task, the following steps are required：

* Firstly, define the CommonEventTemplate regular experssion for matching CommonEvent;
* We should define a router for this event and correspondent handler;
* Traverse all Triggers and match the regular expressions defined in the CommonEvent template. Create Diagnosis if match successfully.

#### 1. Define CommonEventTemplate and Regexp for matching

The commonevent template refer to PrometheusAlertTemplate and KubernetesEventTemplate, the code looks like following:

```go
type CommonEventTemplate struct {
  Regexp CommonEventTemplateRegexp `json:"regexp,omitempty"`
}

type CommonEventTemplateRegexp struct {
  Source    string `json:"source,omitempty"`
  Severity  string `json:"severity,omitempty"`
  Class     string `json:"class,omitempty"`
  Component string `json:"component,omitempty"`
  Group     string `json:"group,omitempty"`
}

type SourceTemplate struct {
+  // CommonEventTemplate specifies the template to create a diagnosis from a common event.
+  // +optional
+  CommonEventTemplate *CommonEventTemplate `json:"common_event_template,omitempty"`
}
```

#### 2. Add a router for receiving common event

```go
// Create commonEventer for managing common events
commonEventer := commoneventer.NewCommonEventer(
	context.Background(),
	ctrl.Log.WithName("commonEventer"),
	mgr.GetClient(),
	mgr.GetCache(),
	featureGate.Enabled(features.CommonEventer),
)

// Add a router for common event
r.HandleFunc("/api/v1/commonevent", commonEventer.Handler)
```

#### 3. Handler

Now, we Implement the logic of this handler by accepting the request and create the common event object or update existing object status.

##### 3.1 Accept request body and deserialize to CommonEventFormat object

```go
body, err := ioutil.ReadAll(r.Body)
if err != nil {
	commonEventDiagnosisGenerationErrorCount.Inc()
	ce.Error(err, "unable to read request body")
	http.Error(w, fmt.Sprintf("unable to read request body: %v", err), http.StatusBadRequest)
	return
}
defer r.Body.Close()
var commonEventFormat CommonEventFormat
err = json.Unmarshal(body, &commonEventFormat)
```

##### 3.2 Create CommonEvent object

The part is same as pagerdutyeventer, following is the code:

```go
var commonEvent diagnosisv1.CommonEvent
if err := ce.client.Get(ce, namespacedName, &commonEvent); err != nil {
  if apierrors.IsNotFound(err) {
	commonEvent = diagnosisv1.CommonEvent{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: diagnosisv1.CommonEventSpec{
			Summary:       commonEventFormat.Summary,
			Source:        commonEventFormat.Source,
			Severity:      commonEventFormat.Severity,
			Timestamp:     commonEventFormat.Timestamp,
			Class:         commonEventFormat.Class,
			Component:     commonEventFormat.Component,
			Group:         commonEventFormat.Group,
			CustomDetails: commonEventFormat.CustomDetails,
		},
	}

	// Create commonEvent in cluster
	err = ce.client.Create(ce, &commonEvent)
	if err != nil {
		commonEventDiagnosisGenerationErrorCount.Inc()
		ce.Error(err, "unable to create Event")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	} else {
		commonEventDiagnosisGenerationErrorCount.Inc()
		ce.Error(err, "unable to fetch Event")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
```

##### 3.3. Match CommonEvent object and tigger

Realize match function while an CommonEvent object is Created in cluster, the fields to match is defined in CommonEventTemplateRegexp. 

```go
func matchCommonEvent(commonEventTemplate diagnosisv1.CommonEventTemplate, commonEvent diagnosisv1.CommonEvent) (bool, error) {
	re, err := regexp.Compile(commonEventTemplate.Regexp.Source)
	if err != nil {
		return false, err
	}
	if !re.MatchString(commonEvent.Spec.Source) {
		return false, nil
	}

	re, err = regexp.Compile(commonEventTemplate.Regexp.Group)
	if err != nil {
		return false, err
	}

	if !re.MatchString(commonEvent.Spec.Group) {
		return false, nil
	}
	re, err = regexp.Compile(commonEventTemplate.Regexp.Class)
	if err != nil {
		return false, err
	}
	if !re.MatchString(commonEvent.Spec.Class) {
		return false, nil
	}

	re, err = regexp.Compile(commonEventTemplate.Regexp.Severity)
	if err != nil {
		return false, err
	}
	if !re.MatchString(commonEvent.Spec.Severity) {
		return false, nil
	}

	re, err = regexp.Compile(commonEventTemplate.Regexp.Component)
	if err != nil {
		return false, err
	}
	if !re.MatchString(commonEvent.Spec.Component) {
		return false, nil
	}
	return true, nil
}
```

##### 3.4 Create diagnosis

We need to generate diagnosis if current CommonEvent object is matched to any trigger in cluster, the members of new diagnosis is determined by trigger Spec.

```go
func (ce *commonEventer) createDiagnosisFromCommonEvent(triggers []diagnosisv1.Trigger, commonEvent diagnosisv1.CommonEvent) (*diagnosisv1.Diagnosis, error) {
	for _, trigger := range triggers {
		sourceTemplate := trigger.Spec.SourceTemplate
		if sourceTemplate.CommonEventTemplate != nil {
			matched, err := matchCommonEvent(*sourceTemplate.CommonEventTemplate, commonEvent)
			if err != nil {
				ce.Error(err, "failed to match trigger and common event")
				continue
			}
			if matched {
				// name and namespace of diagnosis
				now := time.Now()
				name := fmt.Sprintf("%s.%s.%s.%d", CommonEventGeneratedDiagnosisPrefix, commonEvent.Namespace, commonEvent.Name, now.Unix())
				namespace := util.DefautlNamespace
				annotations := make(map[string]string)
				// todo: String()
				//annotations[CommonEventAnnotation] = commonEvent.String()
				diagnosis := diagnosisv1.Diagnosis{
					ObjectMeta: metav1.ObjectMeta{
						Name:        name,
						Namespace:   namespace,
						Annotations: annotations,
					},
					Spec: diagnosisv1.DiagnosisSpec{
						OperationSet: trigger.Spec.OperationSet,
					},
				}
				// fixme: how to determine diagnosis.Spec.NodeName: CommonEvent don not contain this filed
				if trigger.Spec.NodeName != "" {
					diagnosis.Spec.NodeName = trigger.Spec.NodeName
				}
				// Skip if pod reference and node name cannot be determined.
				if diagnosis.Spec.PodReference == nil && diagnosis.Spec.NodeName == "" {
					// todo: String()
					//ce.Info("pod reference and node name cannot be determined for alert", "alert", commonEvent.String())
					ce.Info("pod reference and node name cannot be determined for alert", "alert")
					continue
				}
			}
		}
	}
	return nil, nil
}
```





