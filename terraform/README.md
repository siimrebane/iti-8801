# Week 3 reference configuration

One directory per cloud, `azure/` and `aws/`. Each holds the files every step
needs (`providers.tf`, `variables.tf`, the boot script) and a `steps/` folder
with the resources, one file per step:

| Step | File | Adds |
|---|---|---|
| 1 | `01-network.tf` | the network: VNet/VPC, the three subnets (public, app, db) |
| 2 | `02-walls.tf` | the firewall rules: NSGs / security groups |
| 3 | `03-load-balancer.tf` | the door: public address, load balancer, health probe |
| 4 | `04-way-out.tf` | the NAT gateway, present only while `locked = false` |
| 5 | `05-database.tf` | the managed PostgreSQL server in the db subnet |
| 6 | `06-app-vms.tf` | the two app VMs, booted by cloud-init with the beacon |
| 7 | `07-bastion.tf` | the bastion, the one way in over SSH |

Terraform reads every `.tf` file in the working directory and nothing in
subfolders, so the lab is: copy one step file up, `plan`, read, `apply`, look
at what appeared, next step.

```
cd azure                      # or aws
terraform init
cp steps/01-network.tf .
terraform plan -var-file=me.tfvars
terraform apply -var-file=me.tfvars
cp steps/02-walls.tf .
...
```

The order matters once: `06-app-vms.tf` references the database's hostname,
so `05-database.tf` must be in place first. Everything in `steps/` together is
the complete Assignment 3 configuration; copying all seven at once and applying
is the same result in one plan.

`me.tfvars` (your values, listed in `.gitignore`) and `terraform.tfstate` never
go into git.
