# gograph Deployment Guide

This guide covers various deployment scenarios for gograph, from local development to production environments.

## 📋 Table of Contents

- [Prerequisites](#prerequisites)
- [Local Development](#local-development)
- [Docker Deployment](#docker-deployment)
- [Production Deployment](#production-deployment)
- [Cloud Deployment](#cloud-deployment)
- [Configuration Management](#configuration-management)
- [Monitoring and Logging](#monitoring-and-logging)
- [Troubleshooting](#troubleshooting)

## 🔧 Prerequisites

### System Requirements

- **Operating System**: Linux, macOS, or Windows
- **Go**: Version 1.24 or higher
- **Neo4j**: Version 5.x or higher
- **Memory**: Minimum 2GB RAM (4GB+ recommended for large projects)
- **Storage**: Varies based on project size (typically 100MB-1GB)

### Network Requirements

- **Neo4j**: Port 7687 (Bolt protocol), Port 7474 (HTTP interface)
- **MCP Server**: Port 8080 (configurable)
- **Internet**: Required for dependency downloads and updates

## 🏠 Local Development

### Quick Setup

1. **Install Dependencies**:

   ```bash
   # Install Go (if not already installed)
   # Visit https://golang.org/dl/

   # Install Make (if not already installed)
   # macOS: xcode-select --install
   # Ubuntu/Debian: sudo apt-get install make
   # Windows: Use chocolatey or similar
   ```

2. **Clone and Build**:

   ```bash
   git clone https://github.com/compozy/gograph.git
   cd gograph
   make deps
   make build
   ```

3. **Start Neo4j**:

   ```bash
   # Using Docker (recommended)
   make run-neo4j

   # Or manually
   docker run -d \
     --name gograph-neo4j \
     -p 7474:7474 -p 7687:7687 \
     -e NEO4J_AUTH=neo4j/password \
     -e NEO4J_PLUGINS='["apoc","graph-data-science"]' \
     neo4j:5-community
   ```

4. **Initialize and Test**:

   ```bash
   # Initialize configuration
   ./bin/gograph init

   # Test the setup
   ./bin/gograph analyze --help
   ```

### Development Workflow

```bash
# Start development environment
make dev

# Run tests during development
make test

# Check code quality
make lint

# Build and test changes
make build && make test
```

## 🐳 Docker Deployment

### Using Pre-built Images

```bash
# Pull the latest image
docker pull compozy/gograph:latest

# Run with volume mount
docker run -v $(pwd):/workspace \
  -e NEO4J_URI=bolt://host.docker.internal:7687 \
  -e NEO4J_USERNAME=neo4j \
  -e NEO4J_PASSWORD=password \
  compozy/gograph:latest analyze /workspace
```

### Docker Compose Setup

Create `docker-compose.yml`:

```yaml
version: "3.8"

services:
  neo4j:
    image: neo4j:5-community
    environment:
      NEO4J_AUTH: neo4j/password
      NEO4J_PLUGINS: '["apoc","graph-data-science"]'
    ports:
      - "7474:7474"
      - "7687:7687"
    volumes:
      - neo4j_data:/data
      - neo4j_logs:/logs
    healthcheck:
      test: ["CMD", "cypher-shell", "-u", "neo4j", "-p", "password", "RETURN 1"]
      interval: 30s
      timeout: 10s
      retries: 5

  gograph:
    image: compozy/gograph:latest
    depends_on:
      neo4j:
        condition: service_healthy
    environment:
      NEO4J_URI: bolt://neo4j:7687
      NEO4J_USERNAME: neo4j
      NEO4J_PASSWORD: password
    volumes:
      - ./projects:/workspace
      - ./config:/config
    command: ["serve-mcp", "--config", "/config/gograph.yaml"]
    ports:
      - "8080:8080"

volumes:
  neo4j_data:
  neo4j_logs:
```

Run with:

```bash
docker-compose up -d
```

### Building Custom Images

```bash
# Build the image
docker build -t gograph:local .

# Run the custom image
docker run -it gograph:local --help
```

## 🚀 Production Deployment

### Binary Deployment

1. **Build for Production**:

   ```bash
   # Build optimized binary
   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
     -ldflags "-w -s -X main.Version=$(git describe --tags)" \
     -o gograph cmd/gograph/main.go
   ```

2. **Create Service User**:

   ```bash
   sudo useradd -r -s /bin/false gograph
   sudo mkdir -p /opt/gograph /etc/gograph /var/log/gograph
   sudo chown gograph:gograph /var/log/gograph
   ```

3. **Install Binary**:

   ```bash
   sudo cp gograph /opt/gograph/
   sudo chmod +x /opt/gograph/gograph
   sudo chown gograph:gograph /opt/gograph/gograph
   ```

4. **Create Configuration**:

   ```bash
   sudo tee /etc/gograph/config.yaml > /dev/null <<EOF
   project:
     name: production
     root_path: /data/projects

   neo4j:
     uri: bolt://localhost:7687
     username: neo4j
     password: "${NEO4J_PASSWORD}"
     database: gograph_production

   mcp:
     server:
       port: 8080
       host: 0.0.0.0
     auth:
       enabled: true
       token: "${MCP_AUTH_TOKEN}"
   EOF
   ```

5. **Create Systemd Service**:

   ```bash
   sudo tee /etc/systemd/system/gograph.service > /dev/null <<EOF
   [Unit]
   Description=gograph MCP Server
   After=network.target neo4j.service
   Requires=neo4j.service

   [Service]
   Type=simple
   User=gograph
   Group=gograph
   ExecStart=/opt/gograph/gograph serve-mcp --config /etc/gograph/config.yaml
   Restart=always
   RestartSec=5
   StandardOutput=journal
   StandardError=journal

   # Security settings
   NoNewPrivileges=true
   PrivateTmp=true
   ProtectSystem=strict
   ProtectHome=true
   ReadWritePaths=/var/log/gograph

   [Install]
   WantedBy=multi-user.target
   EOF
   ```

6. **Start Service**:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable gograph
   sudo systemctl start gograph
   sudo systemctl status gograph
   ```

### Kubernetes Deployment

Create `k8s-deployment.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gograph-config
data:
  config.yaml: |
    project:
      name: k8s-deployment
    neo4j:
      uri: bolt://neo4j-service:7687
      username: neo4j
      password: password
    mcp:
      server:
        port: 8080
        host: 0.0.0.0

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gograph
  labels:
    app: gograph
spec:
  replicas: 2
  selector:
    matchLabels:
      app: gograph
  template:
    metadata:
      labels:
        app: gograph
    spec:
      containers:
        - name: gograph
          image: compozy/gograph:latest
          ports:
            - containerPort: 8080
          env:
            - name: NEO4J_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: neo4j-secret
                  key: password
          volumeMounts:
            - name: config
              mountPath: /config
          command: ["gograph", "serve-mcp", "--config", "/config/config.yaml"]
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
      volumes:
        - name: config
          configMap:
            name: gograph-config

---
apiVersion: v1
kind: Service
metadata:
  name: gograph-service
spec:
  selector:
    app: gograph
  ports:
    - port: 8080
      targetPort: 8080
  type: LoadBalancer
```

Deploy with:

```bash
kubectl apply -f k8s-deployment.yaml
```

## ☁️ Cloud Deployment

### AWS Deployment

#### Using ECS Fargate

1. **Create Task Definition**:

   ```json
   {
     "family": "gograph",
     "networkMode": "awsvpc",
     "requiresCompatibilities": ["FARGATE"],
     "cpu": "512",
     "memory": "1024",
     "executionRoleArn": "arn:aws:iam::ACCOUNT:role/ecsTaskExecutionRole",
     "containerDefinitions": [
       {
         "name": "gograph",
         "image": "compozy/gograph:latest",
         "portMappings": [
           {
             "containerPort": 8080,
             "protocol": "tcp"
           }
         ],
         "environment": [
           {
             "name": "NEO4J_URI",
             "value": "bolt://neo4j.cluster.local:7687"
           }
         ],
         "secrets": [
           {
             "name": "NEO4J_PASSWORD",
             "valueFrom": "arn:aws:secretsmanager:region:account:secret:neo4j-password"
           }
         ],
         "logConfiguration": {
           "logDriver": "awslogs",
           "options": {
             "awslogs-group": "/ecs/gograph",
             "awslogs-region": "us-west-2",
             "awslogs-stream-prefix": "ecs"
           }
         }
       }
     ]
   }
   ```

2. **Create Service**:
   ```bash
   aws ecs create-service \
     --cluster gograph-cluster \
     --service-name gograph \
     --task-definition gograph:1 \
     --desired-count 2 \
     --launch-type FARGATE \
     --network-configuration "awsvpcConfiguration={subnets=[subnet-12345],securityGroups=[sg-12345],assignPublicIp=ENABLED}"
   ```

#### Using Lambda (for batch processing)

```python
import json
import subprocess
import boto3

def lambda_handler(event, context):
    # Download gograph binary
    s3 = boto3.client('s3')
    s3.download_file('my-bucket', 'gograph', '/tmp/gograph')

    # Make executable
    subprocess.run(['chmod', '+x', '/tmp/gograph'])

    # Run analysis
    result = subprocess.run([
        '/tmp/gograph', 'analyze',
        '--path', event['project_path'],
        '--project-id', event['project_id']
    ], capture_output=True, text=True)

    return {
        'statusCode': 200,
        'body': json.dumps({
            'stdout': result.stdout,
            'stderr': result.stderr,
            'returncode': result.returncode
        })
    }
```

### Google Cloud Platform

#### Using Cloud Run

1. **Deploy to Cloud Run**:

   ```bash
   # Build and push to Container Registry
   docker build -t gcr.io/PROJECT_ID/gograph .
   docker push gcr.io/PROJECT_ID/gograph

   # Deploy to Cloud Run
   gcloud run deploy gograph \
     --image gcr.io/PROJECT_ID/gograph \
     --platform managed \
     --region us-central1 \
     --allow-unauthenticated \
     --set-env-vars NEO4J_URI=bolt://neo4j-ip:7687 \
     --set-env-vars NEO4J_USERNAME=neo4j \
     --set-secrets NEO4J_PASSWORD=neo4j-password:latest
   ```

### Azure Deployment

#### Using Container Instances

```bash
az container create \
  --resource-group gograph-rg \
  --name gograph \
  --image compozy/gograph:latest \
  --ports 8080 \
  --environment-variables \
    NEO4J_URI=bolt://neo4j.example.com:7687 \
    NEO4J_USERNAME=neo4j \
  --secure-environment-variables \
    NEO4J_PASSWORD=secret123 \
  --command-line "gograph serve-mcp"
```

## ⚙️ Configuration Management

### Environment-based Configuration

```bash
# Development
export GOGRAPH_ENV=development
export GOGRAPH_NEO4J_URI=bolt://localhost:7687
export GOGRAPH_LOG_LEVEL=debug

# Staging
export GOGRAPH_ENV=staging
export GOGRAPH_NEO4J_URI=bolt://staging-neo4j:7687
export GOGRAPH_LOG_LEVEL=info

# Production
export GOGRAPH_ENV=production
export GOGRAPH_NEO4J_URI=bolt://prod-neo4j:7687
export GOGRAPH_LOG_LEVEL=warn
```

### Secrets Management

#### Using HashiCorp Vault

```bash
# Store secrets
vault kv put secret/gograph \
  neo4j_password=secret123 \
  mcp_auth_token=token456

# Retrieve in deployment
export NEO4J_PASSWORD=$(vault kv get -field=neo4j_password secret/gograph)
```

#### Using AWS Secrets Manager

```bash
# Store secret
aws secretsmanager create-secret \
  --name gograph/neo4j \
  --secret-string '{"password":"secret123"}'

# Use in ECS task definition
{
  "name": "NEO4J_PASSWORD",
  "valueFrom": "arn:aws:secretsmanager:region:account:secret:gograph/neo4j:password::"
}
```

## 📊 Monitoring and Logging

### Health Checks

Add health check endpoints:

```go
// In your MCP server
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
    // Check Neo4j connection
    if err := s.neo4j.Ping(); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]string{
            "status": "unhealthy",
            "error": err.Error(),
        })
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "healthy",
    })
}
```

### Logging Configuration

```yaml
# config.yaml
logging:
  level: info
  format: json
  output: /var/log/gograph/app.log
  rotation:
    max_size: 100MB
    max_files: 10
    max_age: 30
```

### Metrics Collection

#### Prometheus Metrics

```go
var (
    requestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "gograph_requests_total",
            Help: "Total number of requests",
        },
        []string{"method", "endpoint"},
    )

    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "gograph_request_duration_seconds",
            Help: "Request duration in seconds",
        },
        []string{"method", "endpoint"},
    )
)
```

### Log Aggregation

#### Using ELK Stack

```yaml
# docker-compose.yml
version: "3.8"
services:
  elasticsearch:
    image: docker.elastic.co/elasticsearch/elasticsearch:8.5.0
    environment:
      - discovery.type=single-node
      - xpack.security.enabled=false
    ports:
      - "9200:9200"

  logstash:
    image: docker.elastic.co/logstash/logstash:8.5.0
    volumes:
      - ./logstash.conf:/usr/share/logstash/pipeline/logstash.conf
    ports:
      - "5044:5044"

  kibana:
    image: docker.elastic.co/kibana/kibana:8.5.0
    ports:
      - "5601:5601"
    environment:
      - ELASTICSEARCH_HOSTS=http://elasticsearch:9200
```

## 🔧 Troubleshooting

### Common Issues

#### Connection Issues

```bash
# Test Neo4j connection
cypher-shell -a bolt://localhost:7687 -u neo4j -p password "RETURN 1"

# Check port availability
netstat -tlnp | grep :7687

# Test from container
docker exec -it gograph-neo4j cypher-shell -u neo4j -p password "RETURN 1"
```

#### Memory Issues

```bash
# Check memory usage
docker stats gograph-container

# Increase memory limits
docker run -m 4g compozy/gograph:latest

# For Kubernetes
resources:
  limits:
    memory: "4Gi"
  requests:
    memory: "2Gi"
```

#### Performance Issues

```bash
# Enable profiling
gograph serve-mcp --pprof --pprof-port 6060

# Analyze performance
go tool pprof http://localhost:6060/debug/pprof/profile
```

### Debugging

#### Enable Debug Logging

```bash
export GOGRAPH_LOG_LEVEL=debug
gograph analyze --verbose
```

#### Database Debugging

```cypher
-- Check database size
CALL apoc.meta.stats()

-- Find slow queries
CALL dbms.listQueries()

-- Check indexes
SHOW INDEXES
```

### Recovery Procedures

#### Database Recovery

```bash
# Backup database
docker exec gograph-neo4j neo4j-admin database dump neo4j

# Restore database
docker exec gograph-neo4j neo4j-admin database load neo4j --from-path=/backups
```

#### Service Recovery

```bash
# Restart service
sudo systemctl restart gograph

# Check logs
sudo journalctl -u gograph -f

# Reset configuration
gograph init --force
```

---

This deployment guide covers the most common deployment scenarios. For specific requirements or custom deployments, refer to the individual component documentation or reach out to the community for support.
