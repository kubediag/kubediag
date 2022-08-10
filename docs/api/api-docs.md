<p>Packages:</p>
<ul>
<li>
<a href="#diagnosis.kubediag.org%2fv1">diagnosis.kubediag.org/v1</a>
</li>
</ul>
<h2 id="diagnosis.kubediag.org/v1">diagnosis.kubediag.org/v1</h2>
Resource Types:
<ul></ul>
<h3 id="diagnosis.kubediag.org/v1.Checkpoint">Checkpoint
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.DiagnosisStatus">DiagnosisStatus</a>)
</p>
<div>
<p>Checkpoint is the checkpoint for resuming unfinished diagnosis.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pathIndex</code><br/>
<em>
int
</em>
</td>
<td>
<p>PathIndex is the index of current path in operation set status.</p>
</td>
</tr>
<tr>
<td>
<code>nodeIndex</code><br/>
<em>
int
</em>
</td>
<td>
<p>NodeIndex is the index of current node in path.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.CommonEvent">CommonEvent
</h3>
<div>
<p>CommonEvent is the Schema for the commonevents API.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.CommonEventSpec">
CommonEventSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>summary</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>source</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>severity</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>timestamp</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>class</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>component</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>group</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>customDetails</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.CommonEventStatus">
CommonEventStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.CommonEventSpec">CommonEventSpec
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.CommonEvent">CommonEvent</a>)
</p>
<div>
<p>CommonEventSpec defines the desired state of CommonEvent.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>summary</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>source</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>severity</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>timestamp</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>class</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>component</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>group</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>customDetails</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.CommonEventStatus">CommonEventStatus
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.CommonEvent">CommonEvent</a>)
</p>
<div>
<p>CommonEventStatus defines the observed state of CommonEvent.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>count</code><br/>
<em>
int
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>resolved</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>diagnosed</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>lastUpdateTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.CronTemplate">CronTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.SourceTemplate">SourceTemplate</a>)
</p>
<div>
<p>CronTemplate specifies the template to create a diagnosis periodically at fixed times.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>schedule</code><br/>
<em>
string
</em>
</td>
<td>
<p>Schedule is the schedule in cron format.
See <a href="https://en.wikipedia.org/wiki/Cron">https://en.wikipedia.org/wiki/Cron</a> for more details.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.Diagnosis">Diagnosis
</h3>
<div>
<p>Diagnosis is the Schema for the diagnoses API.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.DiagnosisSpec">
DiagnosisSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>operationSet</code><br/>
<em>
string
</em>
</td>
<td>
<p>OperationSet is the name of operation set which represents diagnosis pipeline to be executed.</p>
</td>
</tr>
<tr>
<td>
<code>nodeName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>One of NodeName and PodReference must be specified.
NodeName is a specific node which the diagnosis is on.</p>
</td>
</tr>
<tr>
<td>
<code>podReference</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.PodReference">
PodReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PodReference contains details of the target pod.</p>
</td>
</tr>
<tr>
<td>
<code>parameters</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Parameters is a set of the parameters to be passed to opreations.
Parameters and OperationResults are encoded into a json object and sent to operation processor when
running diagnosis.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.DiagnosisStatus">
DiagnosisStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.DiagnosisCondition">DiagnosisCondition
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.DiagnosisStatus">DiagnosisStatus</a>)
</p>
<div>
<p>DiagnosisCondition contains details for the current condition of this diagnosis.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.DiagnosisConditionType">
DiagnosisConditionType
</a>
</em>
</td>
<td>
<p>Type is the type of the condition.</p>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#conditionstatus-v1-core">
Kubernetes core/v1.ConditionStatus
</a>
</em>
</td>
<td>
<p>Status is the status of the condition.
Can be True, False, Unknown.</p>
</td>
</tr>
<tr>
<td>
<code>lastTransitionTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastTransitionTime specifies last time the condition transitioned from one status
to another.</p>
</td>
</tr>
<tr>
<td>
<code>reason</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Reason is a unique, one-word, CamelCase reason for the condition&rsquo;s last transition.</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Message is a human readable message indicating details about last transition.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.DiagnosisConditionType">DiagnosisConditionType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.DiagnosisCondition">DiagnosisCondition</a>)
</p>
<div>
<p>DiagnosisConditionType is a valid value for DiagnosisCondition.Type.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Accepted&#34;</p></td>
<td><p>DiagnosisAccepted means that the diagnosis has been accepted by kubediag agent.</p>
</td>
</tr><tr><td><p>&#34;Complete&#34;</p></td>
<td><p>DiagnosisComplete means the diagnosis has completed its execution.</p>
</td>
</tr><tr><td><p>&#34;OperationNotFound&#34;</p></td>
<td><p>OperationNotFound means the operation is not found when running Diagnosis.</p>
</td>
</tr><tr><td><p>&#34;OperationSetChanged&#34;</p></td>
<td><p>OperationSetChanged means the operation set specification has been changed during diagnosis execution.</p>
</td>
</tr><tr><td><p>&#34;OperationSetNotFound&#34;</p></td>
<td><p>OperationSetNotFound means the operation set is not found when running Diagnosis.</p>
</td>
</tr><tr><td><p>&#34;OperationSetNotReady&#34;</p></td>
<td><p>OperationSetNotReady means the graph has not been updated according to the latest specification.</p>
</td>
</tr></tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.DiagnosisPhase">DiagnosisPhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.DiagnosisStatus">DiagnosisStatus</a>)
</p>
<div>
<p>DiagnosisPhase is a label for the condition of a diagnosis at the current time.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Failed&#34;</p></td>
<td><p>DiagnosisFailed means that all paths in the graph have terminated, and at least one operation in each path
terminated in a failure.</p>
</td>
</tr><tr><td><p>&#34;Pending&#34;</p></td>
<td><p>DiagnosisPending means that the diagnosis has been accepted by the system, but no operation has been started.</p>
</td>
</tr><tr><td><p>&#34;Running&#34;</p></td>
<td><p>DiagnosisRunning means the diagnosis has been bound to a node and one of the operations have been started.
At least one operation is still running.</p>
</td>
</tr><tr><td><p>&#34;Succeeded&#34;</p></td>
<td><p>DiagnosisSucceeded means that all operations in some path have voluntarily terminated with a response code
of 200, and the system is not going to execute rest operations.</p>
</td>
</tr><tr><td><p>&#34;Unknown&#34;</p></td>
<td><p>DiagnosisUnknown means that for some reason the state of the diagnosis could not be obtained, typically due
to an error in communicating with the host of the diagnosis.</p>
</td>
</tr></tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.DiagnosisSpec">DiagnosisSpec
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.Diagnosis">Diagnosis</a>)
</p>
<div>
<p>DiagnosisSpec defines the desired state of Diagnosis.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>operationSet</code><br/>
<em>
string
</em>
</td>
<td>
<p>OperationSet is the name of operation set which represents diagnosis pipeline to be executed.</p>
</td>
</tr>
<tr>
<td>
<code>nodeName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>One of NodeName and PodReference must be specified.
NodeName is a specific node which the diagnosis is on.</p>
</td>
</tr>
<tr>
<td>
<code>podReference</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.PodReference">
PodReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PodReference contains details of the target pod.</p>
</td>
</tr>
<tr>
<td>
<code>parameters</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Parameters is a set of the parameters to be passed to opreations.
Parameters and OperationResults are encoded into a json object and sent to operation processor when
running diagnosis.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.DiagnosisStatus">DiagnosisStatus
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.Diagnosis">Diagnosis</a>)
</p>
<div>
<p>DiagnosisStatus defines the observed state of Diagnosis.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.DiagnosisPhase">
DiagnosisPhase
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Phase is a simple, high-level summary of where the diagnosis is in its lifecycle.
The conditions array, the reason and message fields contain more detail about the
pod&rsquo;s status.
There are five possible phase values:</p>
<p>DiagnosisPending: The diagnosis has been accepted by the system, but no operation has been started.
DiagnosisRunning: The diagnosis has been bound to a node and one of the operations have been started.
At least one operation is still running.
DiagnosisSucceeded: All operations in some path have voluntarily terminated with a response code
of 200, and the system is not going to execute rest operations.
DiagnosisFailed: All paths in the graph have terminated, and at least one operation in each path
terminated in a failure.
DiagnosisUnknown: For some reason the state of the diagnosis could not be obtained, typically due
to an error in communicating with the host of the diagnosis.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.DiagnosisCondition">
[]DiagnosisCondition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Conditions contains current service state of diagnosis.</p>
</td>
</tr>
<tr>
<td>
<code>startTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>StartTime is RFC 3339 date and time at which the object was acknowledged by the system.</p>
</td>
</tr>
<tr>
<td>
<code>failedPaths</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.Path">
[]Path
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>FailedPaths contains all failed paths in diagnosis pipeline.
The last node in the path is the one which fails to execute operation.</p>
</td>
</tr>
<tr>
<td>
<code>succeededPath</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.Path">
Path
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SucceededPath is the succeeded paths in diagnosis pipeline.</p>
</td>
</tr>
<tr>
<td>
<code>operationResults</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>OperationResults contains results of operations.
Parameters and OperationResults are encoded into a json object and sent to operation processor when
running diagnosis.</p>
</td>
</tr>
<tr>
<td>
<code>checkpoint</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.Checkpoint">
Checkpoint
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Checkpoint is the checkpoint for resuming unfinished diagnosis.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.Function">Function
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.Processor">Processor</a>)
</p>
<div>
<p>Function contains the details to run a function as an operation.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>codeSource</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>CodeSource contains the code source files.</p>
</td>
</tr>
<tr>
<td>
<code>runtime</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.FunctionRuntime">
FunctionRuntime
</a>
</em>
</td>
<td>
<p>Runtime is the language to use for writing a function.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.FunctionRuntime">FunctionRuntime
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.Function">Function</a>)
</p>
<div>
<p>FunctionRuntime is a valid value for Function.Runtime.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Python3&#34;</p></td>
<td><p>Python3FunctionRuntime is the runtime for running python3 functions</p>
</td>
</tr></tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.HTTPServer">HTTPServer
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.Processor">Processor</a>)
</p>
<div>
<p>HTTPServer specifies the http server to do operations.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>address</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Address is the serving address of the processor. It must be either an ip or a dns address.
Defaults to kubediag agent advertised address if not specified.</p>
</td>
</tr>
<tr>
<td>
<code>port</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Port is the serving port of the processor.
Defaults to kubediag agent serving port if not specified.</p>
</td>
</tr>
<tr>
<td>
<code>path</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Path is the serving http path of processor.</p>
</td>
</tr>
<tr>
<td>
<code>scheme</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Scheme is the serving scheme of processor. It must be either http or https.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.HostPath">HostPath
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.Storage">Storage</a>)
</p>
<div>
<p>HostPath represents a directory on the host.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>path</code><br/>
<em>
string
</em>
</td>
<td>
<p>Path of the directory on the host.
Defaults to kubediag agent data root if not specified.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.KubernetesEventTemplate">KubernetesEventTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.SourceTemplate">SourceTemplate</a>)
</p>
<div>
<p>KubernetesEventTemplate specifies the template to create a diagnosis from a kubernetes event.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>regexp</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.KubernetesEventTemplateRegexp">
KubernetesEventTemplateRegexp
</a>
</em>
</td>
<td>
<p>Regexp is the regular expression for matching kubernetes event template.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.KubernetesEventTemplateRegexp">KubernetesEventTemplateRegexp
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.KubernetesEventTemplate">KubernetesEventTemplate</a>)
</p>
<div>
<p>KubernetesEventTemplateRegexp is the regular expression for matching kubernetes event template.
All regular expressions must be in the syntax accepted by RE2 and described at <a href="https://golang.org/s/re2syntax">https://golang.org/s/re2syntax</a>.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Name is the regular expression for matching &ldquo;Name&rdquo; of kubernetes event.</p>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Namespace is the regular expression for matching &ldquo;Namespace&rdquo; of kubernetes event.</p>
</td>
</tr>
<tr>
<td>
<code>reason</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Reason is the regular expression for matching &ldquo;Reason&rdquo; of kubernetes event.</p>
</td>
</tr>
<tr>
<td>
<code>message</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Message is the regular expression for matching &ldquo;Message&rdquo; of kubernetes event.</p>
</td>
</tr>
<tr>
<td>
<code>source</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#eventsource-v1-core">
Kubernetes core/v1.EventSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Source is the regular expression for matching &ldquo;Source&rdquo; of kubernetes event.
All fields of &ldquo;Source&rdquo; are regular expressions.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.NamespacedName">NamespacedName
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.PodReference">PodReference</a>)
</p>
<div>
<p>NamespacedName represents a kubernetes api resource.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
<p>Namespace specifies the namespace of a kubernetes api resource.</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name specifies the name of a kubernetes api resource.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.Node">Node
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.OperationSetSpec">OperationSetSpec</a>)
</p>
<div>
<p>Node is a node in the directed acyclic graph. It contains details of the operation.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>id</code><br/>
<em>
int
</em>
</td>
<td>
<em>(Optional)</em>
<p>ID is the unique identifier of the node.
It is identical to node index in adjacency list and set by admission webhook server.</p>
</td>
</tr>
<tr>
<td>
<code>to</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.NodeSet">
NodeSet
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>To is the list of node ids this node links to.</p>
</td>
</tr>
<tr>
<td>
<code>operation</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Operation is the name of operation running on the node.
It is empty if the node is the first in the list.</p>
</td>
</tr>
<tr>
<td>
<code>dependences</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.NodeSet">
NodeSet
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Dependences is the list of depended node ids.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.NodeSet">NodeSet
(<code>[]int</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.Node">Node</a>)
</p>
<div>
<p>NodeSet is the set of node ids.</p>
</div>
<h3 id="diagnosis.kubediag.org/v1.Operation">Operation
</h3>
<div>
<p>Operation is the Schema for the operations API.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.OperationSpec">
OperationSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>processor</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.Processor">
Processor
</a>
</em>
</td>
<td>
<p>Processor describes how to register a operation processor into kubediag.</p>
</td>
</tr>
<tr>
<td>
<code>dependences</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Dependences is the list of all depended operations required to be precedently executed.</p>
</td>
</tr>
<tr>
<td>
<code>storage</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.Storage">
Storage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Storage represents the type of storage for operation results.
Operation results will not be stored if nil.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.OperationSet">OperationSet
</h3>
<div>
<p>OperationSet is the Schema for the operationsets API.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.OperationSetSpec">
OperationSetSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>adjacencyList</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.Node">
[]Node
</a>
</em>
</td>
<td>
<p>AdjacencyList contains all nodes in the directed acyclic graph. The first node in the list represents the
start of a diagnosis.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.OperationSetStatus">
OperationSetStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.OperationSetSpec">OperationSetSpec
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.OperationSet">OperationSet</a>)
</p>
<div>
<p>OperationSetSpec defines the desired state of OperationSet.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>adjacencyList</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.Node">
[]Node
</a>
</em>
</td>
<td>
<p>AdjacencyList contains all nodes in the directed acyclic graph. The first node in the list represents the
start of a diagnosis.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.OperationSetStatus">OperationSetStatus
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.OperationSet">OperationSet</a>)
</p>
<div>
<p>OperationSetStatus defines the observed state of OperationSet.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>paths</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.Path">
[]Path
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Paths is the collection of all directed paths of the directed acyclic graph.</p>
</td>
</tr>
<tr>
<td>
<code>ready</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Specifies whether a valid directed acyclic graph can be generated via provided nodes.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.OperationSpec">OperationSpec
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.Operation">Operation</a>)
</p>
<div>
<p>OperationSpec defines the desired state of Operation.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>processor</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.Processor">
Processor
</a>
</em>
</td>
<td>
<p>Processor describes how to register a operation processor into kubediag.</p>
</td>
</tr>
<tr>
<td>
<code>dependences</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Dependences is the list of all depended operations required to be precedently executed.</p>
</td>
</tr>
<tr>
<td>
<code>storage</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.Storage">
Storage
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Storage represents the type of storage for operation results.
Operation results will not be stored if nil.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.Path">Path
(<code>[]github.com/kubediag/kubediag/api/v1.Node</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.DiagnosisStatus">DiagnosisStatus</a>, <a href="#diagnosis.kubediag.org/v1.OperationSetStatus">OperationSetStatus</a>)
</p>
<div>
<p>Path represents a linear ordering of nodes along the direction of every directed edge.</p>
</div>
<h3 id="diagnosis.kubediag.org/v1.PodReference">PodReference
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.DiagnosisSpec">DiagnosisSpec</a>)
</p>
<div>
<p>PodReference contains details of the target pod.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>NamespacedName</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.NamespacedName">
NamespacedName
</a>
</em>
</td>
<td>
<p>
(Members of <code>NamespacedName</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>container</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Container specifies name of the target container.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.Processor">Processor
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.OperationSpec">OperationSpec</a>)
</p>
<div>
<p>Processor describes how to register a operation processor into kubediag.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>httpServer</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.HTTPServer">
HTTPServer
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>One and only one of the following processor should be specified.
HTTPServer specifies the http server to do operations.</p>
</td>
</tr>
<tr>
<td>
<code>scriptRunner</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.ScriptRunner">
ScriptRunner
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ScriptRunner contains the information to run a script.</p>
</td>
</tr>
<tr>
<td>
<code>function</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.Function">
Function
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Function contains the details to run a function as an operation.</p>
</td>
</tr>
<tr>
<td>
<code>timeoutSeconds</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>Number of seconds after which the processor times out.
Defaults to 30 seconds. Minimum value is 1.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.PrometheusAlertTemplate">PrometheusAlertTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.SourceTemplate">SourceTemplate</a>)
</p>
<div>
<p>PrometheusAlertTemplate specifies the template to create a diagnosis from a prometheus alert.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>regexp</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.PrometheusAlertTemplateRegexp">
PrometheusAlertTemplateRegexp
</a>
</em>
</td>
<td>
<p>Regexp is the regular expression for matching prometheus alert template.</p>
</td>
</tr>
<tr>
<td>
<code>nodeNameReferenceLabel</code><br/>
<em>
github.com/prometheus/common/model.LabelName
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeNameReferenceLabel specifies the label for setting &ldquo;.spec.nodeName&rdquo; of generated diagnosis.
The label value will be set as &ldquo;.spec.nodeName&rdquo; field.</p>
</td>
</tr>
<tr>
<td>
<code>podNamespaceReferenceLabel</code><br/>
<em>
github.com/prometheus/common/model.LabelName
</em>
</td>
<td>
<em>(Optional)</em>
<p>PodNamespaceReferenceLabel specifies the label for setting &ldquo;.spec.podReference.namespace&rdquo; of generated diagnosis.
The label value will be set as &ldquo;.spec.podReference.namespace&rdquo; field.</p>
</td>
</tr>
<tr>
<td>
<code>podNameReferenceLabel</code><br/>
<em>
github.com/prometheus/common/model.LabelName
</em>
</td>
<td>
<em>(Optional)</em>
<p>PodNameReferenceLabel specifies the label for setting &ldquo;.spec.podReference.name&rdquo; of generated diagnosis.
The label value will be set as &ldquo;.spec.podReference.name&rdquo; field.</p>
</td>
</tr>
<tr>
<td>
<code>containerReferenceLabel</code><br/>
<em>
github.com/prometheus/common/model.LabelName
</em>
</td>
<td>
<em>(Optional)</em>
<p>ContainerReferenceLabel specifies the label for setting &ldquo;.spec.podReference.container&rdquo; of generated diagnosis.
The label value will be set as &ldquo;.spec.podReference.container&rdquo; field.</p>
</td>
</tr>
<tr>
<td>
<code>parameterInjectionLabels</code><br/>
<em>
[]github.com/prometheus/common/model.LabelName
</em>
</td>
<td>
<em>(Optional)</em>
<p>ParameterInjectionLabels specifies the labels for setting &ldquo;.spec.parameters&rdquo; of generated diagnosis.
All label names and values will be set as key value pairs in &ldquo;.spec.parameters&rdquo; field.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.PrometheusAlertTemplateRegexp">PrometheusAlertTemplateRegexp
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.PrometheusAlertTemplate">PrometheusAlertTemplate</a>)
</p>
<div>
<p>PrometheusAlertTemplateRegexp is the regular expression for matching prometheus alert template.
All regular expressions must be in the syntax accepted by RE2 and described at <a href="https://golang.org/s/re2syntax">https://golang.org/s/re2syntax</a>.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>alertName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>AlertName is the regular expression for matching &ldquo;AlertName&rdquo; of prometheus alert.</p>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
github.com/prometheus/common/model.LabelSet
</em>
</td>
<td>
<em>(Optional)</em>
<p>Labels is the regular expression for matching &ldquo;Labels&rdquo; of prometheus alert.
Only label values are regular expressions while all label names must be identical to the
prometheus alert label names.</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code><br/>
<em>
github.com/prometheus/common/model.LabelSet
</em>
</td>
<td>
<em>(Optional)</em>
<p>Annotations is the regular expression for matching &ldquo;Annotations&rdquo; of prometheus alert.
Only annotation values are regular expressions while all annotation names must be identical to the
prometheus alert annotation names.</p>
</td>
</tr>
<tr>
<td>
<code>startsAt</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>StartsAt is the regular expression for matching &ldquo;StartsAt&rdquo; of prometheus alert.</p>
</td>
</tr>
<tr>
<td>
<code>endsAt</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>EndsAt is the regular expression for matching &ldquo;EndsAt&rdquo; of prometheus alert.</p>
</td>
</tr>
<tr>
<td>
<code>generatorURL</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>GeneratorURL is the regular expression for matching &ldquo;GeneratorURL&rdquo; of prometheus alert.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.ScriptRunner">ScriptRunner
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.Processor">Processor</a>)
</p>
<div>
<p>ScriptRunner contains the information to run a script.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>script</code><br/>
<em>
string
</em>
</td>
<td>
<p>Script is the content of shell script.</p>
</td>
</tr>
<tr>
<td>
<code>argKeys</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ArgKeys contains a slice of keys in parameters or operationResults. The script arguments are generated
from specified key value pairs.
No argument will be passed to the script if not specified.</p>
</td>
</tr>
<tr>
<td>
<code>operationResultKey</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>OperationResultKey is the prefix of keys to store script stdout, stderr or error message in operationResults.
Execution results will not be updated if not specified.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.SourceTemplate">SourceTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.TriggerSpec">TriggerSpec</a>)
</p>
<div>
<p>SourceTemplate describes the information to generate a diagnosis.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>prometheusAlertTemplate</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.PrometheusAlertTemplate">
PrometheusAlertTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>One and only one of the following source should be specified.
PrometheusAlertTemplate specifies the template to create a diagnosis from a prometheus alert.</p>
</td>
</tr>
<tr>
<td>
<code>kubernetesEventTemplate</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.KubernetesEventTemplate">
KubernetesEventTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>KubernetesEventTemplate specifies the template to create a diagnosis from a kubernetes event.</p>
</td>
</tr>
<tr>
<td>
<code>cronTemplate</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.CronTemplate">
CronTemplate
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>CronTemplate specifies the template to create a diagnosis periodically at fixed times.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.Storage">Storage
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.OperationSpec">OperationSpec</a>)
</p>
<div>
<p>Storage represents the type of storage for operation results.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>hostPath</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.HostPath">
HostPath
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>HostPath represents a directory on the host.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.Trigger">Trigger
</h3>
<div>
<p>Trigger is the Schema for the triggers API.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.TriggerSpec">
TriggerSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>operationSet</code><br/>
<em>
string
</em>
</td>
<td>
<p>OperationSet is the name of referenced operation set in the generated diagnosis.</p>
</td>
</tr>
<tr>
<td>
<code>parameters</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Parameters is a set of the parameters to be passed to diagnosis.</p>
</td>
</tr>
<tr>
<td>
<code>nodeName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeName is the default node which the diagnosis is on.</p>
</td>
</tr>
<tr>
<td>
<code>sourceTemplate</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.SourceTemplate">
SourceTemplate
</a>
</em>
</td>
<td>
<p>SourceTemplate is the template of trigger.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.TriggerStatus">
TriggerStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.TriggerSpec">TriggerSpec
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.Trigger">Trigger</a>)
</p>
<div>
<p>TriggerSpec defines the desired state of Trigger.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>operationSet</code><br/>
<em>
string
</em>
</td>
<td>
<p>OperationSet is the name of referenced operation set in the generated diagnosis.</p>
</td>
</tr>
<tr>
<td>
<code>parameters</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Parameters is a set of the parameters to be passed to diagnosis.</p>
</td>
</tr>
<tr>
<td>
<code>nodeName</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>NodeName is the default node which the diagnosis is on.</p>
</td>
</tr>
<tr>
<td>
<code>sourceTemplate</code><br/>
<em>
<a href="#diagnosis.kubediag.org/v1.SourceTemplate">
SourceTemplate
</a>
</em>
</td>
<td>
<p>SourceTemplate is the template of trigger.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="diagnosis.kubediag.org/v1.TriggerStatus">TriggerStatus
</h3>
<p>
(<em>Appears on:</em><a href="#diagnosis.kubediag.org/v1.Trigger">Trigger</a>)
</p>
<div>
<p>TriggerStatus defines the observed state of Trigger.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>lastScheduleTime</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#time-v1-meta">
Kubernetes meta/v1.Time
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>LastScheduleTime is the last time the cron was successfully scheduled.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
on git commit <code>497899d</code>.
</em></p>
