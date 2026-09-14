# ---------------- the database ----------------

# A Flexible Server on a private network needs a private DNS zone in which
# its hostname resolves to its private address, and a link from that zone
# to the VNet so that the VMs use it.
resource "azurerm_private_dns_zone" "db" {
  name                = "privatelink.postgres.database.azure.com"
  resource_group_name = azurerm_resource_group.rg.name
}

resource "azurerm_private_dns_zone_virtual_network_link" "db" {
  name                  = "${var.prefix}-db-dns-link"
  resource_group_name   = azurerm_resource_group.rg.name
  private_dns_zone_name = azurerm_private_dns_zone.db.name
  virtual_network_id    = azurerm_virtual_network.vnet.id
}

# Managed PostgreSQL. The smallest burstable size; takes about ten minutes
# to create. No public access: it exists only at its private address.
resource "azurerm_postgresql_flexible_server" "db" {
  name                          = "${var.prefix}-db"
  resource_group_name           = azurerm_resource_group.rg.name
  location                      = azurerm_resource_group.rg.location
  version                       = "16"
  sku_name                      = "B_Standard_B1ms"
  storage_mb                    = 32768
  backup_retention_days         = 7
  administrator_login           = "beacon"
  administrator_password        = var.db_password
  delegated_subnet_id           = azurerm_subnet.db.id
  private_dns_zone_id           = azurerm_private_dns_zone.db.id
  public_network_access_enabled = false
  zone                          = "1"

  # The server references the zone, not the link — but it cannot be created
  # before the link exists. An ordering rule the API has and no value shows.
  depends_on = [azurerm_private_dns_zone_virtual_network_link.db]
}


output "db_host" {
  description = "The managed server's hostname; resolves only inside the VNet"
  value       = azurerm_postgresql_flexible_server.db.fqdn
}

