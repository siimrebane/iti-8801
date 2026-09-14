# ---------------- the way out ----------------

# NAT gateway — exists ONLY while locked = false (the build phase). It is the
# app subnet's only route to the internet: apt and the image pull need it.
resource "azurerm_public_ip" "nat" {
  count               = var.locked ? 0 : 1
  name                = "${var.prefix}-nat-ip"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  allocation_method   = "Static"
  sku                 = "Standard"
}

resource "azurerm_nat_gateway" "nat" {
  count               = var.locked ? 0 : 1
  name                = "${var.prefix}-nat"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  sku_name            = "Standard"
}

resource "azurerm_nat_gateway_public_ip_association" "nat" {
  count                = var.locked ? 0 : 1
  nat_gateway_id       = azurerm_nat_gateway.nat[0].id
  public_ip_address_id = azurerm_public_ip.nat[0].id
}

resource "azurerm_subnet_nat_gateway_association" "app" {
  count          = var.locked ? 0 : 1
  subnet_id      = azurerm_subnet.app.id
  nat_gateway_id = azurerm_nat_gateway.nat[0].id
}


output "locked" {
  description = "false = build phase (NAT exists), true = locked down"
  value       = var.locked
}
