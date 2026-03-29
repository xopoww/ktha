output "instance_ip" {
  description = "Public IP of the ktha instance"
  value       = google_compute_instance.ktha.network_interface[0].access_config[0].nat_ip
}
