# thumper

A poor-man's [watcher](https://www.elastic.co/products/watcher), an alert system
which watches for user-defined events happening in an elk stack.

## Runtime configuration

thumper's runtime parameters are specified either on the command-line, in the
environment, or in a configuration file. These parameters will include things
like the elasticsearch address, api keys for pagerduty, etc...

## Alert configuration

Another configuration file (or set of configuration files) is also used, this
one being required and containing all checks which should be performed, as well
as the actions which should be taken should a condition come back true. A single
"alert" encompasses all of the following:

* A time interval to perform the alert check at
* A query to be performed against elasticsearch
* A condition to check the query results against
* A set of actions to take should the condition return true

thumper runs with one or more alerts defined in its configuration, each one
operating independant of the others.

### The file(s)

Alert configuration is defined in one or more yaml files. Each alert comprises
one whole yaml document, like so:

```yaml
# thumper.yml
---
name: alert_foo
# other alert parameters
```

`> thumper -a thumper.yaml`

Multiple alerts could be defined in a single yaml file like so:

```yaml
# thumper.yml
---
name: alert_foo
# other alert parameters

---
name: alert_bar
# other alert parameters
```

`> thumper -a thumper.yaml`

**OR** they could be defined in separate files within the same directory, with
that directory being passed into thumper:

```yaml
# thumper.d/foo.yml
---
name: alert_foo
# other alert parameters
```

and

```yaml
# thumper.d/bar.yml
---
name: alert_bar
# other alert parameters
```

`> thumper -a thumper.d`

### Alert document

A single alert has the following fields in its document (all are required):

```yaml
---
name: something_unique
interval: 5 * * * *
query: # see the query subsection
condition: # see the condition subsection
actions: # see the actions subsection
```

#### name

This is an arbitrary string to identify the alert. It must be unique amongst all
of the defined alerts.

#### interval

A cron-style interval string describing when the query should be run and have
the condition checked against it.

#### query

The query which should be performed against elasticsearch. The results are
simply held onto for the condition check, nothing else is done with them at this
point.

TODO show an example elasticsearch query

#### condition

TODO maybe lua scripting?

#### actions

A list of actions to take should the condition check return true. Each element
in the list is a dict with a `type` key describing the action's type, the rest
of the possible keys differ based on what the type is.

Each action dict, before actually being processed, is run through golang's
`template/text` system with the action's object as the root template object. You
can see examples of using this object in the following subsections.

TODO document available fields in the root template object in the `query`
section, since the condition section will need to use them as well

##### http

Create and execute an http command. A warning is logged if anything except a 2xx
response code is returned.

Example:

```yaml
actions:
    - type: http
      method: POST # optional, defaults to GET
      url: http://example.com/some/endpoint?ARG1=foo
      headers: # optional
        X-Foo: whatever
      body: > # optional
        {
            "name":"{{.Name}}",
            "message":"something terrible has happened"
        }
```

##### pagerduty

Triggers an event in pagerduty. The `--pagerduty-key` param must be set in the
runtime configuration in order to use this action type.

Example:

```yaml
actions:
    - type: pagerduty

      # Defaults to the alert's name. This is used to de-duplicate triggers on
      # pagerduty's end
      #incident_key:

      # While it's possible to use templated terms in here, it makes the most
      # sense to have this be a static key and use the details dict for dynamic
      # data
      description: A short message about the error

      # Optional dict of extra contextual data about this alert
      details:
        foo: "{{.Some.Data}}"
        bar: baz
```

##### lua

TODO not sure if this is going to be a thing. If we do lua scripting for
conditions this should be pretty straightforward to also integrate. Basically
just run an arbitrary lua script as an action.
