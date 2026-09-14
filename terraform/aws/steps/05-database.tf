# ---------------- the database ----------------

# RDS places the instance in one of these subnets; it wants two AZs to
# choose from even when there is one instance.
resource "aws_db_subnet_group" "db" {
  name       = "${var.prefix}-db-subnets"
  subnet_ids = aws_subnet.db[*].id
}

# Managed PostgreSQL. The smallest size (db.t3.micro is the alternative if
# t4g is not offered in your region); takes about ten minutes to create.
# skip_final_snapshot: destroy must not stall on a backup.
resource "aws_db_instance" "db" {
  identifier              = "${var.prefix}-db"
  engine                  = "postgres"
  engine_version          = "16"
  instance_class          = "db.t4g.micro"
  allocated_storage       = 20
  username                = "beacon"
  password                = var.db_password
  db_subnet_group_name    = aws_db_subnet_group.db.name
  vpc_security_group_ids  = [aws_security_group.db.id]
  publicly_accessible     = false
  skip_final_snapshot     = true
  backup_retention_period = 7
}


output "db_host" {
  description = "The managed instance's hostname; resolves to a private address inside the VPC"
  value       = aws_db_instance.db.address
}

