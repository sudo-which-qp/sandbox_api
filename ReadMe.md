## QP Go Backend Template V1

<p>This is a template for me writing go applications for backend services, I created this to make it easy for me to start up a Go application for myself just as how easy it is to just start up a Laravel Application with no stress. This template comes with everything you will need to setup to start building backend applications in Golang.</p>

<p>It uses docker to make sure that you are running an environment that will not give you any issues when you try to run it on any other machine, be in Linux, Windows or Mac OS. I have taken time to make sure it works fine and runs all you will need to just start building instead of setting up.</p>

## What it contains

1. Docker / Docker Compose
2. Dockerfile for production and development
3. Redis
4. MySQL
5. Database Seeding
6. Cron Job
7. Email Queuing
8. phpMyAdmin for viewing database
9. Email Sending Service
10. Air for hot reload of the server
11. Server Graceful Shutdown
12. Rate Limiters
13. Error Notification with Slack
14. Uses JWT
15. Migration for database migrate management

## How to run it

1. Make sure you have Docker running on your machine
2. clone repo
3. run "docker-compose up --build"
4. It will run and build the docker compose file for development

## How to run Go and other commands

1. Open another terminal in the project directory
2. Run "docker ps" to find the docker container that is running the go project
3. Get the ID of the container it should look like this "011d5efb29e3"
4. Now run "docker exec -it 011d5efb29e3 sh"
5. This will give you access to a shell environment in the docker container of the project
