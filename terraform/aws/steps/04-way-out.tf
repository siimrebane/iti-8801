# ---------------- the way out ----------------

# NAT gateway — exists ONLY while locked = false (the build phase).
resource "aws_eip" "nat" {
  count  = var.locked ? 0 : 1
  domain = "vpc"
}

resource "aws_nat_gateway" "nat" {
  count         = var.locked ? 0 : 1
  allocation_id = aws_eip.nat[0].id
  subnet_id     = aws_subnet.public[0].id
}

# The private route table always exists. Its default route through the NAT
# gateway is a resource of its own, gone when locked: THIS is what "no
# internet" means in routing terms. (A route written inside the table block
# would survive the gateway as a dead "blackhole" entry that Terraform then
# ignores; a separate route resource is destroyed cleanly.)
resource "aws_route_table" "private" {
  vpc_id = aws_vpc.vpc.id
}

resource "aws_route" "private_default" {
  count                  = var.locked ? 0 : 1
  route_table_id         = aws_route_table.private.id
  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = aws_nat_gateway.nat[0].id
}

resource "aws_route_table_association" "app" {
  subnet_id      = aws_subnet.app.id
  route_table_id = aws_route_table.private.id
}

resource "aws_route_table_association" "db" {
  count          = 2
  subnet_id      = aws_subnet.db[count.index].id
  route_table_id = aws_route_table.private.id
}


output "locked" {
  description = "false = build phase (NAT exists), true = locked down"
  value       = var.locked
}
