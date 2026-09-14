# Week 2's setup as code — AWS variant — plus what week 2 could not have: a
# managed PostgreSQL instance (RDS) in its own private subnets, a NAT gateway
# that exists only while building, and a bastion as the one way in. An ALB
# needs two public subnets in different AZs, and so does RDS for its private
# subnets. Security groups reference each other instead of CIDRs — note the
# difference from Azure's NSG rules.

data "aws_availability_zones" "azs" {
  state = "available"
}

data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical
  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"]
  }
}

# ---------------- network ----------------

resource "aws_vpc" "vpc" {
  cidr_block           = "10.61.0.0/16"
  enable_dns_hostnames = true
  tags                 = { Name = "${var.prefix}-vpc" }
}

resource "aws_internet_gateway" "igw" {
  vpc_id = aws_vpc.vpc.id
}

# Two public subnets: the ALB needs two AZs. The bastion and (while building)
# the NAT gateway live in the first one.
resource "aws_subnet" "public" {
  count                   = 2
  vpc_id                  = aws_vpc.vpc.id
  cidr_block              = cidrsubnet(aws_vpc.vpc.cidr_block, 8, count.index) # 10.61.0.0/24, 10.61.1.0/24
  availability_zone       = data.aws_availability_zones.azs.names[count.index]
  map_public_ip_on_launch = false
  tags                    = { Name = "${var.prefix}-public-${count.index}" }
}

resource "aws_subnet" "app" {
  vpc_id            = aws_vpc.vpc.id
  cidr_block        = "10.61.10.0/24"
  availability_zone = data.aws_availability_zones.azs.names[0]
  tags              = { Name = "${var.prefix}-app" }
}

# Two database subnets: RDS insists on subnets in at least two AZs, even
# for a single instance.
resource "aws_subnet" "db" {
  count             = 2
  vpc_id            = aws_vpc.vpc.id
  cidr_block        = cidrsubnet(aws_vpc.vpc.cidr_block, 8, 20 + count.index) # 10.61.20.0/24, 10.61.21.0/24
  availability_zone = data.aws_availability_zones.azs.names[count.index]
  tags              = { Name = "${var.prefix}-db-${count.index}" }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.vpc.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.igw.id
  }
}

resource "aws_route_table_association" "public" {
  count          = 2
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

