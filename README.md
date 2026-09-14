# ITI8801 — Cloud Architectures and DevOps

Files handed out in the course. Course site: https://iti8801.digivader.com

| Folder | Week | What |
|---|---|---|
| `terraform/` | 3 | the Assignment 3 reference configuration: one directory per cloud (`azure/`, `aws/`), one `.tf` file per lab step under `steps/`. How to use it: `terraform/README.md` and the practice 3 deck of your cloud. |

```
git clone https://github.com/siimrebane/iti-8801.git
cd iti-8801/terraform/azure      # or terraform/aws
terraform init
```

Your own values (`me.tfvars`) and the state file (`terraform.tfstate`) stay on
your laptop; the `.gitignore` in each directory keeps them out of git.
