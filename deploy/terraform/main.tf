terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
  zone    = var.zone
}

resource "google_compute_instance" "ktha" {
  name         = "ktha-loadtest"
  machine_type = "e2-standard-8"

  boot_disk {
    initialize_params {
      image = "ubuntu-os-cloud/ubuntu-2404-lts-amd64"
      size  = 50
    }
  }

  network_interface {
    network = "default"
    access_config {} # ephemeral public IP
  }

  metadata = {
    ssh-keys = "${var.ssh_user}:${file(var.ssh_public_key_file)}"
  }

  tags = ["ktha"]
}

resource "google_compute_firewall" "ktha" {
  name    = "ktha-allow-ports"
  network = "default"

  allow {
    protocol = "tcp"
    ports    = ["40000", "40001", "30000"]
  }

  source_ranges = ["0.0.0.0/0"]
  target_tags   = ["ktha"]
}

resource "local_file" "ansible_inventory" {
  filename = "${path.module}/../inventory/gcp.yml"
  content  = <<-YAML
    ---
    all:
      hosts:
        ${google_compute_instance.ktha.name}:
          ansible_host: ${google_compute_instance.ktha.network_interface[0].access_config[0].nat_ip}
          ansible_user: ${var.ssh_user}
          ansible_ssh_private_key_file: ${var.ssh_private_key_file}
  YAML
}
