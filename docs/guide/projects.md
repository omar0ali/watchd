# Working with many projects

One daemon manages every project you bring up. You can run `watchd up` in as many
projects as you like, see all of them together, and control any of them from any directory.

```console
$ watchd ls
PROJECT  RUNNING  LOADED  CONFIG
blog     1/1      2h10m   ~/work/blog/watchd.lua
shop     2/3      14m03s  ~/work/shop/watchd.lua

$ watchd ps -a
PROJECT  APP     STATE              PID    UPTIME  RESTARTS
blog     site    running            40211  2h10m   0
shop     api     running (healthy)  41233  14m03s  0
shop     web     running            41260  14m02s  0
shop     worker  stopped            -      -       0
```

## How watchd decides which project you mean

| You run… | It acts on… |
| --- | --- |
| `watchd stop api` inside a project | The project whose `watchd.lua` is in this directory or a parent, the same way git finds its repository |
| `watchd stop -p shop api` | The loaded project named `shop`, from anywhere |
| `watchd stop shop/api blog/site` | Apps in several projects at once, from anywhere |
| `watchd stop api` elsewhere | The only loaded project, if exactly one is loaded |

If watchd can't tell which project you mean, it doesn't guess. It lists the loaded projects
and asks you to pick:

```console
$ watchd stop worker
error: several projects are loaded (blog, shop); choose one with -p NAME, use PROJECT/APP, or run from a project directory
```

## Common tasks

```sh
watchd ls                       # every loaded project
watchd ps -a                    # every app of every project
watchd ps shop                  # the apps of one project
watchd restart shop/api         # restart one app, from anywhere
watchd logs -f blog/site        # follow one app of another project
watchd logs -p shop             # all logs of a project
watchd stop -p shop             # stop every app of a project (it stays loaded)
watchd down blog                # stop and unload one project
watchd down blog shop           # several at once
watchd down --all               # every project
watchd off                      # every project, and the daemon itself
```

`ps` with no arguments shows the current directory's project if it's loaded, and every
project otherwise.

## Project names

Every loaded project needs a unique name, because names are how you address projects from
anywhere.

- The name comes from `name = "..."` in the config, or the config folder's name if that's
  missing (cleaned up: `My App (old)` becomes `my-app-old`).
- Names may contain letters, digits, `.`, `_` and `-`. They can't contain `/`, which
  separates project and app in `PROJECT/APP`.
- If two configs use the same name, the second `watchd up` fails with a message saying which
  config already has it. Rename one of them, or run `watchd down NAME` first.

## up, reload and down

| Command | What happens |
| --- | --- |
| `watchd up` (not loaded yet) | Reads the config, loads the project, starts its apps. |
| `watchd up` (already loaded) | Starts any apps that aren't running. It does **not** re-read the config. |
| `watchd reload` | Re-reads the config and restarts the project with it. Apps that were running come back, and new apps with `autostart = true` start. If the config has errors, nothing changes. |
| `watchd stop` | Stops apps; the project stays loaded and visible in `ps`. |
| `watchd down` | Stops every app and unloads the project. |
