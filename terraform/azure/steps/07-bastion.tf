# ---------------- the bastion ----------------

# One small VM with a public IP in the public subnet. It runs nothing but
# SSH; its only job is to be jumped through (ssh -J).
resource "azurerm_public_ip" "bastion" {
  name                = "${var.prefix}-bastion-ip"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  allocation_method   = "Static"
  sku                 = "Standard"
}

resource "azurerm_network_interface" "bastion" {
  name                = "${var.prefix}-bastion-nic"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location

  ip_configuration {
    name                          = "internal"
    subnet_id                     = azurerm_subnet.public.id
    private_ip_address_allocation = "Static"
    private_ip_address            = "10.60.0.4"
    public_ip_address_id          = azurerm_public_ip.bastion.id
  }
}

resource "azurerm_linux_virtual_machine" "bastion" {
  name                  = "${var.prefix}-bastion"
  resource_group_name   = azurerm_resource_group.rg.name
  location              = azurerm_resource_group.rg.location
  size                  = var.vm_size
  admin_username        = "student"
  network_interface_ids = [azurerm_network_interface.bastion.id]

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

output "bastion_address" {
  description = "The one way in"
  value       = azurerm_public_ip.bastion.ip_address
}

output "ssh_command" {
  description = "Jump through the bastion to app-1"
  value       = "ssh -J student@${azurerm_public_ip.bastion.ip_address} student@${azurerm_network_interface.app[0].private_ip_address}"
}

