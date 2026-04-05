# Deployment Plan: Rugo API to DigitalOcean

Deployment of the Go backend to a DigitalOcean droplet. Since you already have a `Dockerfile`, the most robust approach is using **Docker** with **GitHub Actions** for automated "releases" and deployments via **DigitalOcean Container Registry (DOCR)**.

## 1. Prerequisites
- A DigitalOcean Droplet (Ubuntu recommended).
- Docker and Docker Compose installed on the Droplet.
- A GitHub repository for your code.
- A **DigitalOcean Container Registry** (Starter plan is free).

## 2. CI/CD Workflow (GitHub Actions)
The file `.github/workflows/deploy.yml` automates the release process:
1. **Build & Push**: Builds the image and pushes it to `registry.digitalocean.com/<your-registry>/api:latest`.
2. **Deploy**: SSH into the droplet, logs in to DOCR, pulls the new image, and restarts the backend.

## 3. Configuration Changes
If you are using external Postgres/Redis (e.g. Managed Databases), you'll need to update your `.env` on the server to point to their connection strings.

### Key Environment Variables:
- `DB_ADDR`: `postgres://user:password@host:port/dbname?sslmode=require`
- `REDIS_ADDR`: `host:port`
- `DOCR_NAME`: Your registry name (required for image resolution).

## 4. Implementation Steps

### A. Setup Secrets in GitHub
In your repository settings, add the following secrets:
- `DIGITALOCEAN_ACCESS_TOKEN`: Personal Access Token from DO.
- `DOCR_NAME`: The name of your registry.
- `DROPLET_IP`: The IP address of your DigitalOcean droplet.
- `DROPLET_SSH_KEY`: Your private SSH key.
- `ENV_FILE`: The content of your production `.env` file.

### B. Server Side Setup (Once)
On your droplet:
1. Install Docker: `sudo apt update && sudo apt install docker.io docker-compose -y`
2. Create directory: `mkdir -p ~/rigo-api`
3. Upload `docker-compose.prod.yml` to `~/rigo-api/docker-compose.prod.yml`. Or let the GitHub action handle it.

---

## 5. Summary of Files
- `.github/workflows/deploy.yml`: Automated CI/CD.
- `docker-compose.prod.yml`: Production container config.
- `Dockerfile`: Multi-stage build for a tiny Go binary.
