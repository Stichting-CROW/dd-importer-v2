# Feed Monitor Kubernetes Deployment

Simple Kubernetes deployment for the Feed Monitor service.

## Prerequisites

- Kubernetes cluster
- PostgreSQL database accessible from the cluster
- Telegram bot token and chat ID
- Docker image pushed to registry

## Setup

### 1. Create Required Secrets

Create the database password secret (reuses existing one from main importer):
```bash
# If you already have the main importer secret, skip this
kubectl create secret generic db-password-deelfietsdashboard \
  --from-literal=password='YOUR_DB_PASSWORD'
```

Create the Telegram secrets:
```bash
kubectl create secret generic telegram-bot-credentials \
  --from-literal=bot-token='YOUR_BOT_TOKEN' \
  --from-literal=chat-id='YOUR_CHAT_ID'
```

### 2. Update Deployment

Edit `feed-monitor-deployment.yaml` and update:
- Image URL (line 19)
- Database host (line 22)

### 3. Deploy

```bash
kubectl apply -f k8s/feed-monitor-deployment.yaml
```

### 4. Verify

```bash
kubectl get pods -l app=feed-monitor
kubectl logs -l app=feed-monitor -f
```

## Environment Variables

The deployment expects these secrets to exist:

| Secret Name | Key | Description |
|-------------|-----|-------------|
| `db-password-deelfietsdashboard` | `password` | PostgreSQL password |
| `telegram-bot-credentials` | `bot-token` | Telegram bot token |
| `telegram-bot-credentials` | `chat-id` | Telegram chat ID |

## Updating

```bash
# Update image
kubectl set image deployment/feed-monitor feed-monitor=registry.gitlab.com/bikedashboard/feed-monitor:v1.0.1

# Check status
kubectl rollout status deployment/feed-monitor

# View logs
kubectl logs -l app=feed-monitor -f
```

## Uninstall

```bash
kubectl delete deployment feed-monitor
```
