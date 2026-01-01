# tools.xdoubleu.com

Each tool has it's own folder, however all are combined in cmd/publish.
cmd/publish also takes care of authentication.

The folder structure of tools follow the usual one for go projects:

- images: images used for tool
- migrations: db migrations used for tool
- templates: html templates used for tool
- internal: logic internal to that app
- pkg: logic that could (if I would want to) become their own project
- root level files (?) HTTP endpoints of tools

Existing tools:

## goaltracker

## watchparty

TODO: need room code input template (removed from sign in dto)

TODO: on refresh screen sharing not coming through

TODO: when not sharing a screen show other person camera full screen and own in small corner
TODO: make cameras drag and droppable

TODO: tests
TODO: cleanup code

Ideas:

## own todolist

Todoist is lots of fun and works very nice but I have some specific needs for todolists that they can't deal with:

- Recurring todos can be hidden until I need them
- I want to be able to order by priority and then also DnD todos
- Next to all existing todoist features
- Note: they do have a great app, mobile experience should be goated if I create this
  
## proxy search engine that adds -AI to every google search

I hate AI summaries in Google
