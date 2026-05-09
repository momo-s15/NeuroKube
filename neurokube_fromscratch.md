NeuroKube: step-by-step build checklist

Check off each item: - [ ] → - [x] when that step is done (GitHub/Cursor and many Markdown viewers render these as clickable checkboxes).

Source of truth: [neurokube_blueprint.md](c:\Users\ms11m\vs code projects\SmartKubernetes\neurokube_blueprint.md). Current repo: blueprint only; no code yet.

Architecture (what you are building):

flowchart LR
  subgraph cluster [kind cluster]
    Victim[nginx-victim Deployment]
    Prom[Prometheus]
    AM[Alertmanager]
    Loki[Loki plus Promtail]
    Brain[neurokube-brain Go]
  end
  Host[Host: Ollama]
  Slack[Slack Socket Mode]
  Victim --> Prom
  Prom --> AM
  AM -->|POST /alert| Brain
  Brain --> Loki
  Brain --> Host
  Brain --> Slack
  Slack -->|button| Brain
  Brain -->|client-go patch| Victim

Three highest-risk implementation points (pay extra attention): Part N (wire HTTP + Socket Mode without races — ListenAndServe blocks, Socket in a goroutine), Part P (deployment vs container name — patch body must target container nginx), Part T (Linux/kind: host.docker.internal does not resolve by default — use hostAliases or raw host IP in OLLAMA_URL).

Laptop with ~8 GB GPU VRAM (vs blueprint’s ~12 GB): The blueprint targets an RTX 3060–class GPU. 8 GB VRAM is enough for NeuroKube if you use a smaller Ollama model and align OLLAMA_MODEL in .env and the brain. Diagnoses may be a bit noisier than a big model; the demo pipeline is the same. If the model still won’t load, use Ollama on CPU for the demo (slower, higher system RAM use).



Part A — Prerequisites and host setup





Confirm ~30 GB free disk (blueprint §1). For system RAM, blueprint recommends 16 GB+ for kind + Prometheus + Loki + Ollama comfortably; on a tight laptop close other apps. (8 GB VRAM is separate — see Ollama bullets below.)



Install Docker (Docker Desktop on Windows; ensure WSL2 backend if you use WSL for kind).



Verify Docker runs: docker version.



Install kind v0.22+ (e.g. go install sigs.k8s.io/kind@v0.22.0 on Linux/WSL, or follow kind docs for your OS).



Ensure kind is on your PATH.



Install kubectl 1.29+ and verify kubectl version --client.



Install Helm 3.14+ and verify helm version.



Install Go 1.22+ and verify go version.



Set GOPATH / PATH if needed so go install binaries are found.



Install Ollama on the host (uses GPU when it fits; can fall back to CPU).



Pick a model that fits ~8 GB VRAM: prefer a small tag, e.g. ollama pull llama3.2:3b, or phi3:mini, or gemma2:2b — instead of the blueprint’s default llama3.2 (often a larger variant that may not fit or may be tight on 8 GB). If you have 12+ GB VRAM, ollama pull llama3.2 per the blueprint is fine.



Set OLLAMA_MODEL in .env.example and your real .env to the exact pulled name (e.g. llama3.2:3b); the brain must send the same string to /api/generate.



If loading the model fails with GPU OOM, run the demo with CPU inference (see Ollama docs for your OS — e.g. limit or disable GPU layers) and accept slower responses; increase the Go HTTP client timeout for Ollama if calls time out (see Part V).



Run a quick test: curl http://localhost:11434/api/tags (or Ollama CLI) and a short /api/generate with your chosen model to confirm it runs.



Choose where you will run cluster commands: WSL2 Ubuntu (matches blueprint bash snippets) or native shell; stay consistent for kubectl context.



Part B — Slack application (before tokens in .env)





Create a Slack workspace (free tier is fine) or use an existing dev workspace.



Go to api.slack.com/apps and click Create New App (from scratch).



Name the app (e.g. NeuroKube) and pick the workspace.



In OAuth & Permissions, add Bot Token Scopes: chat:write.



Add Bot Token Scope: chat:write.public.



Add Bot Token Scope: connections:write (required for Socket Mode).



Install to Workspace and copy the Bot User OAuth Token — this becomes SLACK_BOT_TOKEN (starts with xoxb-).



Under Settings → Socket Mode, enable Socket Mode.



Create an App-Level Token with scope connections:write; copy it as SLACK_APP_TOKEN (starts with xapp-).



Under Settings → Interactivity & Shortcuts, enable Interactivity (Socket Mode still delivers block actions).



In Slack, create a channel (e.g. #neurokube-alerts) and invite the bot to the channel if required by your workspace rules.



Note the channel name for SLACK_CHANNEL (e.g. #neurokube-alerts).



Part C — Repository scaffold (files and folders)





Create top-level directory cluster/.



Add cluster/kind-config.yaml exactly as blueprint §2 (1 control-plane with ingress-ready label, 2 workers, host port 30000 mapped to worker).



Add cluster/namespaces.yaml (blueprint lists it but does not define it): manifests for Namespace neurokube and optionally monitoring.



Create observability/.



Add observability/prometheus-values.yaml from blueprint (Grafana enabled, NodePort 30000, admin password, Alertmanager receiver neurokube-webhook URL http://neurokube-brain.neurokube.svc.cluster.local:8080/alert).



Add observability/loki-values.yaml from blueprint (Loki + Promtail).



Create victim/.



Add victim/deployment.yaml from blueprint (nginx + stress sidecar, tight memory limit on nginx).



Add victim/stress-test.sh from blueprint; ensure line endings work in your shell (LF for bash).



Create brain/ as the Go module root (recommended): place go.mod and go.sum here, not at repo root, so docker build ./brain matches the Dockerfile context.



Add brain/main.go (entrypoint from blueprint §3; adjust import path to your real module path, e.g. github.com/you/neurokube/brain or a single module name).



Create brain/handler/, brain/loki/, brain/llm/, brain/k8s/ packages.



Add .env.example at repo root from blueprint §5 (all keys documented, no real secrets).



Add .gitignore including .env, *.exe, and local IDE files.



Add Makefile from blueprint §6; fix paths if go.mod lives under brain/ (e.g. docker build context ./brain).



Add brain/Dockerfile from blueprint §5; if module root is brain/, change build line to go build -o neurokube-brain ./main.go (or ./... entry package).



Add brain/deployment.yaml and brain/rbac.yaml from blueprint §5.



Part D — Day 1: Create cluster





Run kind create cluster --config cluster/kind-config.yaml.



Run kubectl cluster-info --context kind-neurokube.



Run kubectl get nodes and confirm 3 nodes (1 control-plane, 2 workers) are Ready.



Apply cluster/namespaces.yaml if you created it (or rely on Helm --create-namespace later).



Part E — Day 1: Helm repos and observability stack





Add Prometheus community repo: helm repo add prometheus-community https://prometheus-community.github.io/helm-charts.



Add Grafana repo: helm repo add grafana https://grafana.github.io/helm-charts.



Run helm repo update.



Install kube-prometheus-stack: helm install kube-prom-stack prometheus-community/kube-prometheus-stack -n monitoring --create-namespace -f observability/prometheus-values.yaml (adjust release name if you prefer).



Install Loki stack: helm install loki-stack grafana/loki-stack -n monitoring -f observability/loki-values.yaml.



Run kubectl get pods -n monitoring and wait until core pods are Running (may take several minutes).



Run kubectl get svc -n monitoring and note the exact Loki service name and port (blueprint assumes something like loki-stack; chart version may differ — update LOKI_URL later to match).



Part F — Day 1: Grafana and Loki wiring





Open Grafana at http://localhost:30000 (NodePort from kind worker mapping).



Log in with admin / password from prometheus-values.yaml (neurokube123 in blueprint).



Add data source Loki with URL from cluster DNS, e.g. http://<loki-service>.monitoring.svc.cluster.local:3100 (verify port 3100).



Save and test the datasource.



Run Explore → Loki → run a simple query for default namespace logs once the victim pod exists (later step).



Part G — Day 1: Victim workload and stress demo





Apply victim: kubectl apply -f victim/deployment.yaml.



Run kubectl get pods -l app=nginx-victim -n default and wait until Running.



Confirm the pod has two containers (nginx, stress) via kubectl describe pod.



chmod +x victim/stress-test.sh (on Unix/WSL).



Run ./victim/stress-test.sh once; expect pod to OOM / restart.



In Grafana, open metrics Explore and check blueprint §8 style queries (e.g. memory working set, restart count) for nginx-victim.



In Prometheus UI (port-forward to Prometheus if needed), search for ALERTS or OOM-related metrics and see what alertname actually fires for your chart version.



Part H — Day 1: Alertmanager routing (iterate if needed)





Port-forward Alertmanager, e.g. kubectl port-forward svc/kube-prom-stack-alertmanager 9093:9093 -n monitoring (adjust service name to match your Helm release — run kubectl get svc -n monitoring | grep -i alert).



Open http://localhost:9093/#/alerts and confirm which alerts are firing after stress (may need a minute and a fresh stress run).



Read the actual alertname labels shown there. The blueprint uses KubePodOOMKilled; some chart versions fire KubeContainerOOMKilled or other names — your prometheus-values.yaml matchers must match exactly what you see in this UI, not the blueprint string.



Open Alertmanager Status → Config and verify the route sends your real OOM / crash alert names (and optionally KubePodCrashLooping) to receiver neurokube-webhook.



If alert names differ from the blueprint, edit observability/prometheus-values.yaml matchers to match what you saw in the Alertmanager alerts UI.



Run helm upgrade kube-prom-stack prometheus-community/kube-prometheus-stack -n monitoring -f observability/prometheus-values.yaml to apply changes.



Expect webhook failures (connection refused, 502, timeouts) until the brain pod exists — that is expected and fine on Day 1; you are validating routing and alert names, not a healthy webhook yet.



Part I — Day 2: Go module and dependencies





cd brain (module root).



go mod init <your-module-path>.



go get github.com/slack-go/slack@latest.



go get k8s.io/client-go@v0.29.0 (or align all k8s.io/* to same minor).



go get k8s.io/apimachinery@v0.29.0.



Optionally go get github.com/gorilla/mux or use net/http only.



Run go mod tidy.



Part J — Day 2: Config and types





Define handler.Config struct with fields: Port, LokiURL, OllamaURL, OllamaModel, SlackBotToken, SlackAppToken, SlackChannel, KubeConfig (blueprint §3).



Implement getEnv(key, fallback string) string in main.go or a small config helper.



In main, build Config from os.Getenv with defaults matching blueprint (fix LOKI_URL default if service name differs).



Part K — Day 2: Loki client (discover labels before LogQL)





Implement loki/Client with BaseURL and FetchLogs(namespace, pod string, lines int).



Port-forward Loki to localhost when testing from the host, e.g. kubectl port-forward svc/<loki-service> 3100:3100 -n monitoring (use the real service name from kubectl get svc -n monitoring).



Run curl 'http://localhost:3100/loki/api/v1/labels' and note which label keys exist.



Run curl 'http://localhost:3100/loki/api/v1/label/pod/values' and see if pod names appear.



If pod is empty or missing, try curl 'http://localhost:3100/loki/api/v1/label/pod_name/values' — Promtail / chart version may use pod_name instead of pod.



Write LogQL that uses the actual label names you discovered (e.g. {namespace="default", pod=~"nginx-victim.*"} or {namespace="default", pod_name=~"nginx-victim.*"}); do not assume the blueprint query works until verified.



Call /loki/api/v1/query_range with limit, start, end, direction=backward.



Parse JSON response and concatenate log lines from data.result[].values.



Return a clear string when no logs are found.



Test: curl the same query_range URL your Go client will use and compare results to logs from the running brain.



Part L — Day 2: Ollama LLM client (stay on /api/generate)





Keep the stack Ollama-only as in the blueprint: use POST /api/generate with JSON body model, prompt, stream: false — do not switch to /api/chat unless you intentionally change the integration later. Use cfg.OllamaModel (from OLLAMA_MODEL) so an 8 GB VRAM laptop can run llama3.2:3b (or similar) without code changes.



Implement llm/Client with BaseURL, Model.



Add the system prompt from blueprint §3 requiring strict JSON: root_cause, evidence, fix, new_limit.



Decode the top-level response field from the Ollama JSON response (generate API shape).



Test: call from host with sample alert text and confirm JSON-shaped output (may need prompt tuning).



Part M — Day 2: Alertmanager HTTP handler





Define AlertmanagerPayload and Alert structs (blueprint §3).



Implement handleAlert: decode JSON, respond 400 on bad body, return 200 quickly.



For each alert with status == "firing", spawn go s.processAlert(alert) so HTTP handler does not block.



Implement processAlert: read pod, namespace, alertname from labels; log receipt.



Call s.loki.FetchLogs(namespace, pod, 50); on error, use fallback string and continue.



Call s.llm.Diagnose(alertName, pod, logs); on error, set diagnosis string and continue.



Initially log the diagnosis only; later wire s.slack.SendCrashAlert(...).



Part N — Day 2: Server glue (recommended pattern — avoids lifecycle races)





Define handler.Server struct holding config, Loki client, LLM client, Slack client, K8s client references.



Implement NewServer(cfg Config) *Server that constructs all clients (K8s may be nil until Part P).



Wire HTTP with http.NewServeMux: register POST /alert → s.handleAlert (use method check inside handler or a small wrapper).



Implement Start() so ListenAndServe blocks and startSocketMode runs in a background goroutine — this keeps main alive without fighting two blocking loops:

func (s *Server) Start() error {
    mux := http.NewServeMux()
    mux.HandleFunc("/alert", s.handleAlert)

    go s.startSocketMode() // Socket Mode in background; must call socketmode client Run inside this goroutine

    return http.ListenAndServe(":"+s.cfg.Port, mux)
}





Inside startSocketMode, use socketmode.New and client.Run() in this same goroutine (the goroutine started from Start); do not start a second goroutine that exits immediately without Run.



main should call log.Fatal(srv.Start()) (or log and exit on error) so the process stays up while serving HTTP.



Part O — Day 2: Run brain locally (no Slack yet)





Export KUBECONFIG to your kind kubeconfig path.



Export LOKI_URL to a reachable URL (port-forward Loki if brain runs on host).



Export OLLAMA_URL=http://localhost:11434.



Run go run . from brain/ (or go run ./...).



Send blueprint §7 test curl to http://localhost:8080/alert with a fake payload.



Confirm logs show Loki fetch + LLM call (or clear errors for each step).



Part P — Day 3: Kubernetes client and patch (blueprint bug: container name ≠ app label)





Implement k8s.NewClient: if kubeconfig path empty, use rest.InClusterConfig(); else clientcmd.BuildConfigFromFlags("", kubeconfig).



Implement PatchMemoryLimit(namespace, podName, newLimit string): get the Pod by name; resolve the Deployment name (e.g. from pod labels app: nginx-victim or owner references — deployment name may match app for this victim, but that is not the container name).



Critical: the blueprint wrongly used the app label value as the container name. For the victim manifest, the nginx container is named nginx, not nginx-victim. The merge patch containers array must use name: "nginx" (or resolve the target container name from the live pod spec if you generalize later).



Build the patch body so only that container’s resources.limits.memory is updated — the stress sidecar must remain untouched. Example fragment for the deployment strategic merge:

"containers": []map[string]interface{}{
    {
        "name": "nginx", // container name, NOT the app label / deployment name
        "resources": map[string]interface{}{
            "limits": map[string]string{"memory": newLimit},
        },
    },
},





Call Deployments(namespace).Patch with the patch type you chose (e.g. MergePatchType with the full spec.template.spec.containers structure as required by the API).



Optionally implement RestartPod from blueprint (not always needed if deployment rollout handles it).



Test locally: run brain with kubeconfig, exercise patch against kind, or compare kubectl get deploy nginx-victim -o yaml before/after.



Part Q — Day 3: Slack message blocks





Define Diagnosis struct matching JSON fields from LLM (new_limit etc.).



Implement SendCrashAlert: try json.Unmarshal on raw LLM output; on failure, fall back to plain text in RootCause.



Build Block Kit: header, sections for pod/alert, root cause, evidence, fix, divider, action block.



Primary button apply_patch with value = fmt.Sprintf("%s|%s|%s", ns, pod, newLimit).



Danger button dismiss with same or simpler value.



Post to SLACK_CHANNEL with PostMessage + MsgOptionBlocks.



Part R — Day 3: Slack Socket Mode and interactions





Implement SlackClient wrapper storing api *slack.Client and channel ID or name.



Implement feedback to the user after a button click using ResponseURL (POST JSON to that URL for ephemeral-style updates) or slack-go helpers — the URL you need is not on the raw socket event at top level; it lives on the interaction callback (next steps).



Implement startSocketMode using github.com/slack-go/slack/socketmode.



On EventTypeInteractive, call client.Ack(*evt.Request) immediately.



Blueprint gap: handleInteraction in the blueprint does not show unpacking. For block button actions, response_url, action_id, and value live inside the interaction callback, not at the top level of socketmode.Event.



Type-assert socket event payload data to slack.InteractionCallback, e.g. callback, ok := evt.Data.(slack.InteractionCallback); if !ok, log and return.



Read fields from the callback (slack-go shape is roughly):

callback, ok := evt.Data.(slack.InteractionCallback)
if !ok {
    return
}
actionID := callback.ActionCallback.BlockActions[0].ActionID
value := callback.ActionCallback.BlockActions[0].Value
responseURL := callback.ResponseURL





Before indexing [0], check len(callback.ActionCallback.BlockActions) > 0 (and optionally match ActionID explicitly for apply_patch / dismiss).



Dispatch to handleButtonAction(actionID, value, responseURL) (or equivalent) so patch success/failure posts use responseURL from this callback.



For apply_patch, call s.k8s.PatchMemoryLimit and post success/failure using the callback’s response mechanism.



For dismiss, post confirmation the same way.



Ensure bot token and app token are set in environment when testing.



Part S — Day 3: Docker image and kind load





From repo root, docker build -t neurokube-brain:latest ./brain (or path where Dockerfile lives).



Fix Dockerfile COPY and RUN go build paths for your module layout.



Run kind load docker-image neurokube-brain:latest --name neurokube.



Confirm image is visible to cluster (docker exec into kind node and crictl images if troubleshooting).



Part T — Day 3: Secrets, RBAC, deploy brain (Linux: host.docker.internal)





Copy .env.example to .env and fill real tokens (never commit .env).



Set OLLAMA_URL=http://host.docker.internal:11434 in .env for pods when that hostname resolves (typical on Windows/macOS Docker Desktop).



On Linux, host.docker.internal often does not resolve inside kind pods. Cleanest fix: add hostAliases under the brain pod spec in brain/deployment.yaml so the name resolves to the Docker bridge gateway (verify IP on your machine — 172.17.0.1 is the common default):

spec:
  hostAliases:
    - ip: "172.17.0.1"   # verify: ip route | grep docker (or docker network inspect)
      hostnames:
        - "host.docker.internal"





Alternative to hostAliases: set OLLAMA_URL to the host’s LAN IP (or the bridge IP) directly in the Secret — no fake DNS name.



kubectl create namespace neurokube (or apply namespaces YAML).



kubectl create secret generic neurokube-secrets --from-env-file=.env -n neurokube (or Makefile dry-run apply pattern).



kubectl apply -f brain/rbac.yaml.



kubectl apply -f brain/deployment.yaml.



Wait for pod Running: kubectl get pods -n neurokube -w.



kubectl logs -n neurokube -l app=neurokube-brain -f and confirm no crash loop (fix Ollama URL / hostAliases / IP if connection errors).



Part U — Day 3: End-to-end demo





Confirm Alertmanager webhook URL resolves to brain Service DNS inside cluster.



Run ./victim/stress-test.sh again (or make demo).



Watch brain logs: should show alert, Loki, Ollama, Slack post.



Check Slack channel for the rich message.



Click Apply Patch; confirm ephemeral success in Slack.



Run kubectl get deployment nginx-victim -n default -o yaml and verify nginx container memory limit increased.



Confirm new pod is stable after rollout.



Click Dismiss on a future test alert and confirm no patch applied.



Part V — Optional polish and troubleshooting





Add ServiceMonitor / PodMonitor so Prometheus scrapes the brain (up{job="neurokube-brain"} in blueprint §8).



Import Grafana dashboards mentioned in blueprint §8 (IDs 15760, 13639) if desired.



If alerts never fire, re-check Prometheus rules and kube-state-metrics pod labels.



If Loki is empty, check Promtail DaemonSet pods and scraped paths.



If Ollama times out, increase HTTP client timeout in Go or use smaller prompt.



If Slack button no-ops, verify Socket Mode token scopes, that client.Run() is executing, and that evt.Data is unpacked as slack.InteractionCallback with BlockActions populated (Part R).



If patch returns 403, re-check ClusterRole verbs and ServiceAccount binding.



Record a short demo video (blueprint §9 Day 3) for portfolio.



Quick reference — blueprint gaps to remember





Implement Server / NewServer / routes / Socket Mode loop / handleInteraction yourself (blueprint shows fragments only); prefer go startSocketMode() + blocking ListenAndServe (Part N).



Patch container name nginx, not the app label / deployment selector string (Part P).



Discover Loki labels with /loki/api/v1/labels and pod vs pod_name before fixing LogQL (Part K).



Match Alertmanager matchers to actual firing alertname values from http://localhost:9093/#/alerts (e.g. KubeContainerOOMKilled vs KubePodOOMKilled) (Part H).



Linux kind: hostAliases or raw host IP for Ollama URL (Part T).



Ollama: keep /api/generate and response field per blueprint (Part L); OLLAMA_MODEL must match a model that fits your VRAM (e.g. :3b on 8 GB) or CPU fallback.



Slack Socket Mode: unpack slack.InteractionCallback from evt.Data; use callback.ResponseURL and callback.ActionCallback.BlockActions[0] for ActionID / Value (Part R) — not top-level event fields.



Keep go.mod inside brain/ if Dockerfile build context is ./brain (avoids path confusion).



Work through parts in order; skip Part V on first pass if you want the fastest vertical slice. Toggle - [ ] → - [x] in the editor as you finish each bullet.