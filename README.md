# README #

This README would normally document whatever steps are necessary to get your application up and running.

### What is this repository for? ###

* Quick summary: A library to interact with databases in a consistent way across projects

* Version
* [Learn Markdown](https://bitbucket.org/tutorials/markdowndemo)

### How do I get set up? ###

### Contribution guidelines ###



### Who do I talk to? ###



### Install packages:

* go get -u github.com/golang-migrate/migrate/v4
* go get -u github.com/golang-migrate/migrate/v4/database/postgres
* go get -u github.com/golang-migrate/migrate/v4/source/file


### How to uninstall packages:
 
 * go clean -i -n github.com/golang-migrate/migrate...

### Create new migration files - from within db folder (not migrations folder)
 * migrate create -ext sql -dir migrations -seq create_user_image_table

### Build Project
 * make build


### Format code
 * make fmt


### Linter Aggregator
 * curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s latest
 * make lint

### Upgrade library in org after pushing to remote
 * git tag v0.1.0
 * git push origin v0.1.0