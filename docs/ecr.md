# ECR — IAM Role Authentication

Amazon ECR uses short-lived tokens rather than static credentials. The workflow is:

1. **Assume an IAM role** (via OIDC, instance profile, or pod identity)
2. **Exchange it for an ECR token** (`aws ecr get-login-password`)
3. **Pass `AWS` as the username and the token as the password** to `helm-semver`

---

## Required IAM permissions

The role assumed by your CI runner needs the following permissions. Scope the
`Resource` to specific repository ARNs in production.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "ECRAuth",
      "Effect": "Allow",
      "Action": "ecr:GetAuthorizationToken",
      "Resource": "*"
    },
    {
      "Sid": "ECRPush",
      "Effect": "Allow",
      "Action": [
        "ecr:BatchCheckLayerAvailability",
        "ecr:InitiateLayerUpload",
        "ecr:UploadLayerPart",
        "ecr:CompleteLayerUpload",
        "ecr:PutImage",
        "ecr:DescribeRepositories"
      ],
      "Resource": "arn:aws:ecr:<region>:<account-id>:repository/helm-charts/*"
    }
  ]
}
```

> [!IMPORTANT]
> ECR repositories must exist before you push. Unlike GHCR, ECR does not
> auto-create repositories on first push. Run `aws ecr create-repository`
> once per chart, e.g:
> ```bash
> aws ecr create-repository \
>   --repository-name helm-charts/observability \
>   --region eu-west-1
> ```

---

## GitHub Actions — OIDC (no stored secrets)

The recommended approach. GitHub's OIDC provider exchanges a short-lived JWT
for AWS temporary credentials — no `AWS_ACCESS_KEY_ID` stored as a secret.

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

      - name: Configure AWS credentials via OIDC
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: arn:aws:iam::123456789012:role/helm-semver-release
          aws-region: eu-west-1

      - name: Get ECR login token
        id: ecr
        run: |
          TOKEN=$(aws ecr get-login-password --region eu-west-1)
          echo "::add-mask::${TOKEN}"
          echo "token=${TOKEN}" >> $GITHUB_OUTPUT

      - uses: rmcneill/helm-semver@v1
        with:
          registry: oci://123456789012.dkr.ecr.eu-west-1.amazonaws.com/helm-charts
          registry-type: oci
          registry-username: AWS
          registry-password: ${{ steps.ecr.outputs.token }}
          github-token: ${{ secrets.GITHUB_TOKEN }}
```

**Trust policy for the IAM role** — allows GitHub Actions OIDC for your repo:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "token.actions.githubusercontent.com:aud": "sts.amazonaws.com",
          "token.actions.githubusercontent.com:sub": "repo:my-org/my-repo:ref:refs/heads/main"
        }
      }
    }
  ]
}
```

---

## GitLab CI — OIDC

```yaml
release-charts:
  image: ghcr.io/rmcneill/helm-semver:latest
  id_tokens:
    AWS_TOKEN:
      aud: sts.amazonaws.com
  before_script:
    - apk add --no-cache aws-cli
    - >
      aws sts assume-role-with-web-identity
      --role-arn arn:aws:iam::123456789012:role/helm-semver-release
      --role-session-name gitlab-ci
      --web-identity-token "$AWS_TOKEN"
      --query "Credentials.[AccessKeyId,SecretAccessKey,SessionToken]"
      --output text | read AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN
    - export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN
    - ECR_TOKEN=$(aws ecr get-login-password --region eu-west-1)
  script:
    - helm-semver release
        --registry oci://123456789012.dkr.ecr.eu-west-1.amazonaws.com/helm-charts
        --registry-username AWS
        --registry-password "$ECR_TOKEN"
  only:
    - main
```

---

## Bitbucket Pipelines — OIDC

Bitbucket supports AWS OIDC natively via the
[`oidc` pipe](https://bitbucket.org/atlassian/aws-assume-role-with-web-identity).

```yaml
pipelines:
  branches:
    main:
      - step:
          name: Release Charts
          oidc: true
          image: ghcr.io/rmcneill/helm-semver:latest
          script:
            - pipe: atlassian/aws-assume-role-with-web-identity:1.0.0
              variables:
                AWS_REGION: eu-west-1
                ROLE_ARN: arn:aws:iam::123456789012:role/helm-semver-release
            - export ECR_TOKEN=$(aws ecr get-login-password --region eu-west-1)
            - helm-semver release
                --registry oci://123456789012.dkr.ecr.eu-west-1.amazonaws.com/helm-charts
                --registry-username AWS
                --registry-password "$ECR_TOKEN"
```

---

## Self-hosted runners / EKS pod identity

If your runner already has an IAM role attached (EC2 instance profile or EKS
pod identity), no explicit credential configuration is needed. The AWS CLI
picks up the role automatically from the instance metadata service (IMDS):

```yaml
- name: Get ECR token
  id: ecr
  run: |
    TOKEN=$(aws ecr get-login-password --region eu-west-1)
    echo "::add-mask::${TOKEN}"
    echo "token=${TOKEN}" >> $GITHUB_OUTPUT

- uses: rmcneill/helm-semver@v1
  with:
    registry: oci://123456789012.dkr.ecr.eu-west-1.amazonaws.com/helm-charts
    registry-username: AWS
    registry-password: ${{ steps.ecr.outputs.token }}
```

---

## Token lifetime

ECR tokens are valid for **12 hours**. This is more than sufficient for any CI
job but you must re-fetch the token on each run — do not cache it between
pipeline executions.
