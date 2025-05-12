# CHIRPY
 Boot.dev project

## .env
1. don't commit changes
  1.  `git update-index --skip-worktree .env` 
1. if necessary to add variables
  1. update values with template values (don't store secrets)
  1. `git update-index --no-skip-worktree .env`
  1. add and commit
  1. `git update-index --skip-worktree .env`

