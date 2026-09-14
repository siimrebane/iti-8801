# ---------------- the machines ----------------

# Two app VMs behind the load balancer: count = 2 makes two of everything
# below, addressed app[0] and app[1]. Private IPs .10 and .11.
resource "azurerm_network_interface" "app" {
  count               = 2
  name                = "${var.prefix}-app-${count.index + 1}-nic"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location

  ip_configuration {
    name                          = "internal"
    subnet_id                     = azurerm_subnet.app.id
    private_ip_address_allocation = "Static"
    private_ip_address            = cidrhost("10.60.1.0/24", 10 + count.index)
    # Note what is MISSING here: public_ip_address_id.
  }
}

resource "azurerm_network_interface_backend_address_pool_association" "app" {
  count                   = 2
  network_interface_id    = azurerm_network_interface.app[count.index].id
  ip_configuration_name   = "internal"
  backend_address_pool_id = azurerm_lb_backend_address_pool.app.id
}

# The boot script with its holes filled: the database's hostname comes from
# the database block below, so the VMs are created after the server exists.
locals {
  app_cloud_init = base64encode(templatefile("${path.module}/cloud-init-app.yaml", {
    beacon_image = var.beacon_image
    db_target    = "${azurerm_postgresql_flexible_server.db.fqdn}:5432"
  }))
}

resource "azurerm_linux_virtual_machine" "app" {
  count                 = 2
  name                  = "${var.prefix}-app-${count.index + 1}"
  resource_group_name   = azurerm_resource_group.rg.name
  location              = azurerm_resource_group.rg.location
  size                  = var.vm_size
  admin_username        = "student"
  network_interface_ids = [azurerm_network_interface.app[count.index].id]
  custom_data           = local.app_cloud_init

  admin_ssh_key {
    username   = "student"
    public_key = var.admin_ssh_key
  }

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = "Standard_LRS"
  }

  source_image_reference {
    publisher = "Canonical"
    offer     = "ubuntu-24_04-lts"
    sku       = "server"
    version   = "latest"
  }
}


output "app_private_ips" {
  value = azurerm_network_interface.app[*].private_ip_address
}

