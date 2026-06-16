# ACR — Workload Identity / Service Principal Authentication

Azure Container Registry supports several authentication methods. For CI
pipelines, **workload identity** (OIDC) is recommended — no secrets stored,
tokens are short-lived and scoped to the job.

---

## Authentication methods

| Method | Recommended for | Username | Password |
|--------|----------------|----------|----------|
| Workload identity (OIDC) | GitHub Actions, AKS | `00000000-0000-0000-0000-000000000000` | short-lived token from `az acr login` |
| Service principal | GitLab CI, Bitbucket, non-Azure runners | SP client ID | SP client secret |
| Admin credentials | Local dev only | registry name | from Azure portal |

> [!NOTE]
> The username `00000000-0000-0000-0000-000000000000` for workload identity is
> a fixed placeholder hardcoded by Azure — it is not your tenant or client ID.
> ACR only validates the short-lived token, not the username string.

---

## Required RBAC

Assign the `AcrPush` role to your managed identity or service principal,
scoped to the registry:

```bash
az role assignment create \
  --assignee <client-id-or-managed-identity-principal-id> \
  --role AcrPush \
  --scope /subscriptions/<sub-id>/resourceGroups/<rg>/providers/Microsoft.ContainerRegistry/registries/<registry-name>
```

---

## GitHub Actions — workload identity (OIDC)

No secrets stored. Azure federated credentials exchange GitHub's OIDC JWT for
a short-lived ACR token.

**1. Configure a federated credential on your managed identity:**

```bash
az identity federated-credential create \
  --name github-actions \
  --identity-name helm-semver-release \
  --resource-group my-rg \
  --issuer https://token.actions.githubusercontent.com \
  --subject repo:my-org/my-repo:ref:refs/heads/main \
  --audiences api://AzureADTokenExchange
```

**2. GitHub Actions workflow:**

```yaml
jobs:
  release:
    runs-on: ubuntu-latest
    permissions:
      contents: write
      id-token: write    # required for OIDC

    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Azure login (OIDC)
        uses: azure/login@v2
        with:
          client-id: ${{ secrets.AZURE_CLIENT_ID }}
          tenant-id: ${{ secrets.AZURE_TENANT_ID }}
          subscription-id: ${{ secrets.AZURE_SUBSCRIPTION_ID }}

      - name: Get ACR token
        id: acr
        run: |
          TOKEN=$(az acr login \
            --name myregistry \
            --expose-token \
            --output tsv \
            --query accessToken)
          echo "::add-mask::${TOKEN}"
          echo "token=${TOKEN}" >> $GITHUB_OUTPUT

      - uses: rhysmcneill/helm-semver@v1
        with:
          registry: oci://myregistry.azurecr.io/helm-charts
          registry-type: oci
          registry-username: "00000000-0000-0000-0000-000000000000"
          registry-password: ${{ steps.acr.outputs.token }}
          github-token: ${{ secrets.GITHUB_TOKEN }}
```

Secrets required: `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID` —
none of these are credentials, they are identifiers only.

---

## GitLab CI — service principal

```yaml
release-charts:
  image: ghcr.io/rhysmcneill/helm-semver:latest
  before_script:
    - apk add --no-cache azure-cli
    - az login --service-principal
        --username $AZURE_CLIENT_ID
        --password $AZURE_CLIENT_SECRET
        --tenant $AZURE_TENANT_ID
    - export ACR_TOKEN=$(az acr login
        --name myregistry
        --expose-token
        --output tsv
        --query accessToken)
  script:
    - helm-semver release
        --registry oci://myregistry.azurecr.io/helm-charts
        --registry-username "00000000-0000-0000-0000-000000000000"
        --registry-password "$ACR_TOKEN"
  only:
    - main
```

Store `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, and `AZURE_TENANT_ID` as
GitLab CI/CD variables (Settings → CI/CD → Variables), masking the secret.

---

## Bitbucket Pipelines — service principal

```yaml
pipelines:
  branches:
    main:
      - step:
          name: Release Charts
          image: ghcr.io/rhysmcneill/helm-semver:latest
          script:
            - az login --service-principal
                --username $AZURE_CLIENT_ID
                --password $AZURE_CLIENT_SECRET
                --tenant $AZURE_TENANT_ID
            - export ACR_TOKEN=$(az acr login
                --name myregistry
                --expose-token
                --output tsv
                --query accessToken)
            - helm-semver release
                --registry oci://myregistry.azurecr.io/helm-charts
                --registry-username "00000000-0000-0000-0000-000000000000"
                --registry-password "$ACR_TOKEN"
```

Store credentials as Bitbucket repository variables
(Settings → Repository variables), marking `AZURE_CLIENT_SECRET` as secured.

---

## AKS — pod-managed identity

If your runner is a pod in AKS with a managed identity attached, skip the
explicit login entirely — `az acr login` will use the pod's identity
automatically:

```yaml
- name: Get ACR token
  id: acr
  run: |
    TOKEN=$(az acr login \
      --name myregistry \
      --expose-token \
      --output tsv \
      --query accessToken)
    echo "::add-mask::${TOKEN}"
    echo "token=${TOKEN}" >> $GITHUB_OUTPUT

- uses: rhysmcneill/helm-semver@v1
  with:
    registry: oci://myregistry.azurecr.io/helm-charts
    registry-username: "00000000-0000-0000-0000-000000000000"
    registry-password: ${{ steps.acr.outputs.token }}
```

---

## Token lifetime

ACR tokens issued via `az acr login --expose-token` are valid for **3 hours**.
Always re-fetch the token on each pipeline run — do not cache between
executions.
