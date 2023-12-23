run "test01" {
  assert {
    condition = local_file.helloworld.content == "hello world!"
    error_message = "error message" 
  }
}

run "test02" {
  assert {
    condition = local_file.helloworld.content == "hello world!"
    error_message = "error message" 
  }
}
