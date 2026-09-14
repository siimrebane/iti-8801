terraform {
  required_version = ">= 1.5"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
  }
}

provider "azurerm" {
  features {}
  # azurerm 4.x needs to be told which subscription; az login alone is not enough.
  subscription_id = var.subscription_id

  # A subscription can use a service only after its resource provider is
  # registered, once. Compute, network and storage were registered the first
  # time the portal touched them. PostgreSQL was not: Terraform registers it
  # here, at plan time, so nobody has to do it by hand.
  resource_provider_registrations = "core"
  resource_providers_to_register  = ["Microsoft.DBforPostgreSQL"]
}
