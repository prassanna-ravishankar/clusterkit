resource "google_sql_database_instance" "main" {
  name             = var.instance_name
  project          = var.project_id
  region           = var.region
  database_version = var.database_version

  settings {
    tier              = var.tier
    availability_type = var.availability_type
    disk_size         = var.disk_size
    disk_type         = var.disk_type

    ip_configuration {
      ipv4_enabled    = var.ipv4_enabled
      private_network = var.private_network
    }

    backup_configuration {
      enabled                        = var.backup_enabled
      start_time                     = var.backup_start_time
      point_in_time_recovery_enabled = var.point_in_time_recovery_enabled
      transaction_log_retention_days = var.transaction_log_retention_days
    }

    maintenance_window {
      day          = var.maintenance_window_day
      hour         = var.maintenance_window_hour
      update_track = var.maintenance_window_update_track
    }

    database_flags {
      name  = "max_connections"
      value = var.max_connections
    }
  }

  deletion_protection = var.deletion_protection
}

resource "google_sql_database" "databases" {
  for_each = toset(var.databases)

  name     = each.value
  instance = google_sql_database_instance.main.name
  project  = var.project_id
}

resource "google_sql_user" "users" {
  for_each = nonsensitive(var.users)

  name     = each.key
  instance = google_sql_database_instance.main.name
  project  = var.project_id
  password = each.value.password
}
