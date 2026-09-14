variable "prefix" {
  description = "Name prefix for every resource, e.g. your student code (lowercase)"
  type        = string
}

variable "region" {
  description = "The region you used in weeks 1 and 2, e.g. eu-central-1"
  type        = string
}

variable "beacon_image" {
  description = "Course beacon container image"
  type        = string
  default     = "ghcr.io/siimrebane/iti8801-beacon:a3"
}

variable "db_password" {
  description = "Master password of the managed PostgreSQL instance (not graded, just don't use 'postgres')"
  type        = string
  sensitive   = true
}

variable "admin_ssh_key" {
  description = "Your SSH public key: goes on the bastion and the app instances. The bastion is the only way to use it."
  type        = string
}

variable "my_ip" {
  description = "Your public IPv4, e.g. 193.40.12.34: the only address allowed to SSH to the bastion"
  type        = string
}

# THE variable of this lab. false = build phase: a NAT gateway gives the app
# subnet outbound access so user-data can install Docker and pull the image.
# true = locked: NAT gateway destroyed, no route to the internet, and the
# beacon's own report proves it.
variable "locked" {
  type    = bool
  default = false
}

# The free-plan size; t3.micro is the alternative where t4g/t3 availability
# differs. Same size for the bastion.
variable "instance_type" {
  type    = string
  default = "t3.micro"
}
