# ---------------- the bastion ----------------

# One small instance with a public IP in the public subnet. It runs nothing
# but SSH; its only job is to be jumped through (ssh -J).
resource "aws_instance" "bastion" {
  ami                         = data.aws_ami.ubuntu.id
  instance_type               = var.instance_type
  subnet_id                   = aws_subnet.public[0].id
  associate_public_ip_address = true
  vpc_security_group_ids      = [aws_security_group.bastion.id]
  key_name                    = aws_key_pair.admin.key_name
  tags                        = { Name = "${var.prefix}-bastion" }
}

output "bastion_address" {
  description = "The one way in"
  value       = aws_instance.bastion.public_ip
}

output "ssh_command" {
  description = "Jump through the bastion to app-1"
  value       = "ssh -J ubuntu@${aws_instance.bastion.public_ip} ubuntu@${aws_instance.app[0].private_ip}"
}

