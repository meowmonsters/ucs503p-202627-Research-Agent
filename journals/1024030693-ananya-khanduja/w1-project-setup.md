# Week 1 : Project Setup and Frontend Integration

## Objective

The objective for Week 1 was to set up the project repository, understand its structure, and integrate the initial frontend application into the team's codebase.

## Repository Setup

The project repository was cloned locally from GitHub.

```bash
git clone https://github.com/meowmonsters/ucs503p-202627-Research-Agent.git
cd ucs503p-202627-Research-Agent
After cloning the repository, I checked the current branch and verified that the project was using the master branch.

Understanding the Repository Structure

I explored the existing repository structure to understand where the different components of the project were located.

The main directories included:

code/ - source code
code/backend/ - existing backend code
assets/ - project assets
docs/ - documentation
journals/ - individual team member journals
project-proposal/ - project proposal
project-report-final/ - final project report

The frontend was to be integrated inside the code/ directory.

Frontend Integration

The provided Vite + React + TypeScript frontend starter was copied into the project repository.

Git Workflow

After adding the frontend files, I checked the repository status using:

git status

The newly added frontend directory appeared as an untracked directory.

I then staged the frontend files using:

git add code/analyst-agent-web/

A commit was created with the message:

feat: initialize Vite + React + TypeScript project for garv

The commit successfully added the initial frontend files to the repository.

Team Coordination

The frontend changes were prepared and pushed before the backend changes so that the backend contributor could pull the latest version of the repository before adding their work.

This helped maintain an organized Git workflow between team members.

Key Learnings
Learned how to clone and work with a GitHub repository locally.
Learned how to pull the latest changes before starting work.
Understood the organization of an existing project repository.
Learned the basic structure of a Vite + React + TypeScript project.
Learned how to stage files and create commits using Git.
Understood the importance of coordinating Git changes when working in a team.
Outcome

The initial Vite + React + TypeScript frontend was successfully integrated into the team's repository under:

code/analyst-agent-web/

The changes were committed and pushed successfully, providing the initial frontend structure for further development.