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
* A search to be performed against elasticsearch
* A condition to check the search results against
* A set of actions to take should the condition return true

thumper runs with one or more alerts defined in its configuration, each one
operating independant of the others.

### The file(s)

Alert configuration is defined in one or more yaml files. Each file contains an
array of alerts, like so

```yaml
# thumper.yml
- name: alert_foo
  # other alert parameters

- name: alert_bar
  # other alert parameters
```

`> thumper -a thumper.yaml`

**OR** they could be defined in separate files within the same directory, with
that directory being passed into thumper:

```yaml
# thumper.d/foo.yml
- name: alert_foo
  # other alert parameters

- name: alert_foo2
  # other alert parameters
```

and

```yaml
# thumper.d/bar.yml
- name: alert_bar
  # other alert parameters

- name: alert_bar2
  # other alert parameters
```

`> thumper -a thumper.d`

### Alert document

A single alert has the following fields in its document (all are required):

```yaml
---
name: something_unique
interval: "5 * * * *"
search: # see the search subsection
condition: # see the condition subsection
actions: # see the actions subsection
```

#### name

This is an arbitrary string to identify the alert. It must be unique amongst all
of the defined alerts.

#### interval

A cron-style interval string describing when the search should be run and have
the condition checked against it.

#### search

The search which should be performed against elasticsearch. The results are
simply held onto for the condition check, nothing else is done with them at this
point.

```yaml
# conveniently, json is valid yaml
search: {
        "query": {
            "query_string": {
                "query":"severity:fatal"
            }
        }
}
```

As a shortcut, since a `query_string` is used so often, the abofe can also
simply be written as:

```yaml
search: "severity:fatal"
```

See the [query dsl][querydsl] docs for more on how to formulate query objects.
See the [query string][querystring] docs for more on how to formulate query
strings.

#### condition

Once the search is performed the results are checked against this step to
determine if they warrant performing the alert's actions. Conditionals are
defined as lua scripts, either files or simply inline with the yaml itself. The
search result's data can be accessed through the `ctx` global variable. See the
alert context section for all available fields in `ctx`.

```yaml
condition:
    lua_file: ./foo-condition.yml
```

**OR**

```yaml
condition:
    lua_inline: |
        if ctx.HitCount > 10 then
            return true
        end
        return false
```

#### actions

A list of actions to take should the condition check return true. Each element
in the list is a dict with a `type` key describing the action's type, the rest
of the possible keys differ based on what the type is.

Each action dict, before actually being processed, is run through golang's
`template/text` system with the action's context object as the root template object. You
can see examples of using this object in the following subsections. See the
alert context section for all fields available in the context.

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

Similar to condition, lua can be used to perform virtually any action you might
think of. Also, as in condition, the `ctx` global variable will be made
available with all the result data from the search.

Example:

```
actions:
    - type: lua
      lua_file: ./some-action.yml
```

**OR**

```
actions:
    - type: lua
      lua_inline: |
        -- do some lua stuff here
        -- no need to return anything
```

*Note that the go templating will still be applied to the lua action definition
before it is processed. It's therefore possible to incorporate go template
entities into your `lua_inline`. This is not recommended, as this would cause
unbounded growth in the lua function cache, and the exact same data is available
in `ctx` anyway. It's also possible to dynamically load different `lua_file`s
depending on various conditions. It's not clear why this would be useful, but
it's probably fine to do*

### Alert context

Through its lifecycle each alert has a context object attached to it. The
results from the search step are included in it, as well as other data. Here is
a description of the available data in the context, as well as how to use it

*NOTE THAT THE CONTEXT IS READ-ONLY IN ALL CASES*

#### Context fields

// Hit describes one of the documents matched by a search query
type Hit struct {
}

```
{
    Name      string // The alert's name
    StartedTS uint64 // The timestamp the alert started at

    // The following are filled in by the search step
    TookMS      uint64  // Time search took to complete, in milliseconds
    HitCount    uint64  // The total number of documents matched
    HitMaxScore float64 // The maximum score of all the documents matched

    // Array of actual documents matched
    Hits []{
        Index  string  // The index the hit came from
        Type   string  // The type the document is
        ID     string  // The unique id of the document
        Score  float64 // The document's score relative to the query
        Source object  // The actual document
    }
}
```

#### In lua

Within lua scripts the context is made available as a global variable called
`ctx`. Fields on it are directly addressable using the above names, for example
`ctx.HitCount` and `ctx.Hits[1].ID`.

#### In go template

In some areas go templates, provided by the `template/text` package. In these
places the context is made available as the root object. For example,
`{{.HitCount}}`.

[querydsl]: https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl.html
[querystring]: https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-query-string-query.html#query-string-syntax
