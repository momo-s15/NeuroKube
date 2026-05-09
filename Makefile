.PHONY: cluster-up cluster-down obs-install brain-build brain-deploy demo demo-oom clean all logs

cluster-up:
	kind create cluster --config cluster/kind-config.yaml
	kubectl cluster-info --context kind-neurokube

cluster-down:
	kind delete cluster --name neurokube

obs-install:
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
	helm repo add grafana https://grafana.github.io/helm-charts
	helm repo update
	helm install kube-prom-stack prometheus-community/kube-prometheus-stack \
	  --namespace monitoring --create-namespace \
	  --values observability/prometheus-values.yaml
	helm install loki-stack grafana/loki-stack \
	  --namespace monitoring \
	  --values observability/loki-values.yaml

brain-build:
	docker build -t neurokube-brain:latest ./brain
	kind load docker-image neurokube-brain:latest --name neurokube

brain-deploy:
	kubectl apply -f cluster/namespaces.yaml
	@python -c "open('.env.k8s.tmp','w',newline='').writelines(l for l in open('.env',encoding='utf-8') if not l.lstrip().startswith('KUBECONFIG='))"
	kubectl create secret generic neurokube-secrets \
	  --from-env-file=.env.k8s.tmp --namespace neurokube \
	  --dry-run=client -o yaml | kubectl apply -f -
	@python -c "import pathlib; pathlib.Path('.env.k8s.tmp').unlink(missing_ok=True)"
	kubectl apply -f brain/rbac.yaml
	kubectl apply -f brain/deployment.yaml
	kubectl apply -f brain/servicemonitor.yaml
	kubectl apply -f victim/deployment.yaml

demo:
	@echo "==> Triggering OOMKill demo..."
	bash victim/stress-test.sh

demo-oom:
	@echo "==> Applying OOM-oriented victim manifest (stress as PID 1). Revert: kubectl apply -f victim/deployment.yaml"
	kubectl apply -f victim/deployment-oom.yaml
	kubectl rollout status deployment/nginx-victim -n default --timeout=120s

logs:
	kubectl logs -n neurokube -l app=neurokube-brain -f

clean:
	kind delete cluster --name neurokube
	-docker rmi neurokube-brain:latest

all: cluster-up obs-install brain-build brain-deploy
	@echo "==> NeuroKube is live. Run: make demo"
