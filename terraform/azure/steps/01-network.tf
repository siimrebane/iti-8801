# Week 2's setup as code, plus what week 2 could not have: a managed
# PostgreSQL server in its own private subnet, a NAT gateway that exists only
# while building, and a bastion as the one way in. Same shape you built by
# hand: VNet, subnets, NSGs as the walls, a load balancer as the only door,
# two VMs without public IPs.

resource "azurerm_resource_group" "rg" {
  name     = "${var.prefix}-rg"
  location = var.location
}

# ---------------- network ----------------

resource "azurerm_virtual_network" "vnet" {
  name                = "${var.prefix}-vnet"
  resource_group_name = azurerm_resource_group.rg.name
  location            = azurerm_resource_group.rg.location
  address_space       = ["10.60.0.0/16"]
}

# The public subnet holds exactly one thing: the bastion.
resource "azurerm_subnet" "public" {
  name                 = "public"
  resource_group_name  = azurerm_resource_group.rg.name
  virtual_network_name = azurerm_virtual_network.vnet.name
  address_prefixes     = ["10.60.0.0/24"]
}

resource "azurerm_subnet" "app" {
  name                 = "app"
  resource_group_name  = azurerm_resource_group.rg.name
  virtual_network_name = azurerm_virtual_network.vnet.name
  address_prefixes     = ["10.60.1.0/24"]
  # No default outbound access: the VM leaves only via a public IP, a NAT
  # gateway or LB outbound. Same as the "Private subnet" checkbox in the portal.
  default_outbound_access_enabled = false
}

# The database subnet is DELEGATED to the Flexible Server service: the
# service places its server here, and nothing else may live in this subnet.
resource "azurerm_subnet" "db" {
  name                 = "db"
  resource_group_name  = azurerm_resource_group.rg.name
  virtual_network_name = azurerm_virtual_network.vnet.name
  address_prefixes     = ["10.60.2.0/24"]
  # The Flexible Server service adds this endpoint to its subnet by itself
  # when the server is created. Declared here, so that the next plan does not
  # see it as drift and remove it.
  service_endpoints = ["Microsoft.Storage"]

  delegation {
    name = "postgres"
    service_delegation {
      name    = "Microsoft.DBforPostgreSQL/flexibleServers"
      actions = ["Microsoft.Network/virtualNetworks/subnets/join/action"]
    }
  }
}

