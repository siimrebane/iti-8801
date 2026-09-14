# ---------------- the machines ----------------

# Your SSH public key, registered once and used by every instance.
resource "aws_key_pair" "admin" {
  key_name   = "${var.prefix}-key"
  public_key = var.admin_ssh_key
}

# Two app instances behind the ALB: count = 2, private IPs .10 and .11.
# The boot script's holes are filled with the database's hostname, so the
# instances are created after the database exists.
resource "aws_instance" "app" {
  count                  = 2
  ami                    = data.aws_ami.ubuntu.id
  instance_type          = var.instance_type
  subnet_id              = aws_subnet.app.id
  private_ip             = cidrhost("10.61.10.0/24", 10 + count.index)
  vpc_security_group_ids = [aws_security_group.app.id]
  key_name               = aws_key_pair.admin.key_name
  user_data = templatefile("${path.module}/user-data-app.sh", {
    beacon_image = var.beacon_image
    db_target    = "${aws_db_instance.db.address}:5432"
  })
  tags = { Name = "${var.prefix}-app-${count.index + 1}" }
}


resource "aws_lb_target_group_attachment" "app" {
  count            = 2
  target_group_arn = aws_lb_target_group.app.arn
  target_id        = aws_instance.app[count.index].id
  port             = 80
}


output "app_private_ips" {
  value = aws_instance.app[*].private_ip
}

