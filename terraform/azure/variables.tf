variable "prefix" {
  description = "Name prefix for every resource, e.g. your student code (lowercase)"
  type        = string
}

variable "subscription_id" {
  description = "Your subscription id: az account show --query id -o tsv"
  type        = string
}

variable "location" {
  description = "The region you used in weeks 1 and 2, e.g. swedencentral"
  type        = string
}

variable "beacon_image" {
  description = "Course beacon container image"
  type        = string
  default     = "ghcr.io/siimrebane/iti8801-beacon:a3"
}

variable "db_password" {
  description = "Administrator password of the managed PostgreSQL server (not graded, just don't use 'postgres')"
  type        = string
  sensitive   = true
}

variable "admin_ssh_key" {
  description = "Your SSH public key: goes on the bastion and the app VMs. The bastion is the only way to use it."
  type        = string
}

variable "my_ip" {
  description = "Your public IPv4, e.g. 193.40.12.34: the only address allowed to SSH to the bastion"
  type        = string
}

# THE variable of this lab. false = build phase: a NAT gateway gives the app
# subnet outbound access so cloud-init can install Docker and pull the image.
# true = locked: the NAT gateway is destroyed, the subnet has no route to the
# internet, and the beacon's own report proves it.
variable "locked" {
  type    = bool
  default = false
}

# Free-tier B-series size differs by region: B2ats_v2 in newer regions, B1s in
# older ones. Pick the one "See all sizes" marks free in yours.
variable "vm_size" {
  type    = string
  default = "Standard_B2ats_v2"
}
