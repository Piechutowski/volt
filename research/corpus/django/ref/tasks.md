# Tasks

<div class="versionadded">

6.0

</div>

<div class="module" synopsis="Django's built-in background Task system.">

django.tasks

</div>

The Task framework provides the contract and plumbing for background work, not the engine that runs it. The Tasks API defines how work is described, queued, and tracked, but leaves actual execution to external infrastructure.

## Task definition

### The `task` decorator

<div class="function">

task(*, priority=0, queue_name="default", backend="default", takes_context=False,*\*kwargs)

The `@task` decorator defines a `Task` instance. All keyword arguments are passed directly to the backend's `task_class` (which defaults to `Task`).

The following standard arguments are supported:

- `priority`: Sets the `~Task.priority` of the `Task`. Defaults to 0.
- `queue_name`: Sets the `~Task.queue_name` of the `Task`. Defaults to `"default"`.
- `backend`: Sets the `~Task.backend` of the `Task`. Defaults to `"default"`.
- `takes_context`: Controls whether the `Task` function accepts a `TaskContext`. Defaults to `False`. See `Task context
  <task-context>` for details.

Custom Task backends may define a custom `task_class` that accepts additional arguments. These can be passed through the `@task` decorator:

    @task(foo=5, bar=600)
    def my_task():
        pass

<div class="versionchanged">

6.1

Support for passing arbitrary `**kwargs` to the `@task` decorator is added.

</div>

If the defined `Task` is not valid according to the backend, `~django.tasks.exceptions.InvalidTask` is raised.

See `defining tasks <defining-tasks>` for usage examples.

</div>

### `Task`

<div class="Task">

Represents a Task to be run in the background. Tasks should be defined using the `task` decorator.

Attributes of `Task` cannot be modified. See `modifying Tasks
<modifying-tasks>` for details.

<div class="attribute">

Task.priority

The priority of the `Task`. Priorities must be between -100 and 100, where larger numbers are higher priority, and will be run sooner.

The backend must have `.supports_priority` set to `True` to use this feature.

</div>

<div class="attribute">

Task.backend

The alias of the backend the `Task` should be enqueued to. This must match a backend defined in `BACKEND <TASKS-BACKEND>`.

</div>

<div class="attribute">

Task.queue_name

The name of the queue the `Task` will be enqueued on to. Defaults to `"default"`. This must match a queue defined in `QUEUES <TASKS-QUEUES>`, unless `QUEUES <TASKS-QUEUES>` is set to `[]`.

</div>

<div class="attribute">

Task.run_after

The earliest time the `Task` will be executed. This can be a `timedelta <datetime.timedelta>`, which is used relative to the current time, a timezone-aware `datetime <datetime.datetime>`, or `None` if not constrained. Defaults to `None`.

This attribute can be set using `~Task.using`.

The backend must have `.supports_defer` set to `True` to use this feature. Otherwise, `~django.tasks.exceptions.InvalidTask` is raised.

</div>

<div class="attribute">

Task.name

The name of the function decorated with `task`. This name is not necessarily unique.

</div>

<div class="method">

Task.using(\*, priority=None, backend=None, queue_name=None, run_after=None)

Creates a new `Task` with modified defaults. The existing `Task` is left unchanged.

`using` allows modifying the following attributes:

- `priority <Task.priority>`
- `backend <Task.backend>`
- `queue_name <Task.queue_name>`
- `run_after <Task.run_after>`

See `modifying Tasks <modifying-tasks>` for usage examples.

</div>

<div class="method">

Task.enqueue(*args,*\*kwargs)

Enqueues the `Task` to the `Task` backend for later execution.

Arguments are passed to the `Task`'s function after a round-trip through a `json.dumps`/`json.loads` cycle. Hence, all arguments must be JSON-serializable and preserve their type after the round-trip.

If the `Task` is not valid according to the backend, `~django.tasks.exceptions.InvalidTask` is raised.

See `enqueueing Tasks <enqueueing-tasks>` for usage examples.

</div>

<div class="method">

Task.aenqueue(*args,*\*kwargs)

The `async` variant of `enqueue <Task.enqueue>`.

</div>

<div class="method">

Task.get_result(result_id)

Retrieves a result by its id.

If the result does not exist, `TaskResultDoesNotExist
<django.tasks.exceptions.TaskResultDoesNotExist>` is raised. If the result is not the same type as the current Task, `TaskResultMismatch <django.tasks.exceptions.TaskResultMismatch>` is raised. If the backend does not support `get_result()`, `NotImplementedError` is raised.

</div>

<div class="method">

Task.aget_result(*args,*\*kwargs)

The `async` variant of `get_result <Task.get_result>`.

</div>

</div>

## Task context

<div class="TaskContext">

Contains context for the running `Task`. Context only passed to a `Task` if it was defined with `takes_context=True`.

Attributes of `TaskContext` cannot be modified.

<div class="attribute">

TaskContext.task_result

The `TaskResult` currently being run.

</div>

<div class="attribute">

TaskContext.attempt

The number of the current execution attempts for this Task, starting at 1.

</div>

</div>

## Task results

<div class="TaskResultStatus">

An Enum representing the status of a `TaskResult`.

<div class="attribute">

TaskResultStatus.READY

The `Task` has just been enqueued, or is ready to be executed again.

</div>

<div class="attribute">

TaskResultStatus.RUNNING

The `Task` is currently being executed.

</div>

<div class="attribute">

TaskResultStatus.FAILED

The `Task` raised an exception during execution, or was unable to start.

</div>

<div class="attribute">

TaskResultStatus.SUCCESSFUL

The `Task` has finished executing successfully.

</div>

</div>

<div class="TaskResult">

The `TaskResult` stores the information about a specific execution of a `Task`.

Attributes of `TaskResult` cannot be modified.

<div class="attribute">

TaskResult.task

The `Task` the result was enqueued for.

</div>

<div class="attribute">

TaskResult.id

A unique identifier for the result, which can be passed to `Task.get_result`.

The format of the id will depend on the backend being used. Task result ids are always strings less than 64 characters.

See `Task results <task-results>` for more details.

</div>

<div class="attribute">

TaskResult.status

The `status <TaskResultStatus>` of the result.

</div>

<div class="attribute">

TaskResult.enqueued_at

The time when the `Task` was enqueued.

</div>

<div class="attribute">

TaskResult.started_at

The time when the `Task` began execution, on its first attempt.

</div>

<div class="attribute">

TaskResult.last_attempted_at

The time when the most recent `Task` run began execution.

</div>

<div class="attribute">

TaskResult.finished_at

The time when the `Task` finished execution, whether it failed or succeeded.

</div>

<div class="attribute">

TaskResult.backend

The backend the result is from.

</div>

<div class="attribute">

TaskResult.errors

A list of `TaskError` instances for the errors raised as part of each execution of the Task.

</div>

<div class="attribute">

TaskResult.return_value

The return value from the `Task` function.

If the `Task` did not finish successfully, `ValueError` is raised.

See `return values <task-return-values>` for usage examples.

</div>

<div class="method">

TaskResult.refresh

Refresh the result's attributes from the queue store.

</div>

<div class="method">

TaskResult.arefresh

The `async` variant of `TaskResult.refresh`.

</div>

<div class="attribute">

TaskResult.is_finished

Whether the `Task` has finished (successfully or not).

</div>

<div class="attribute">

TaskResult.attempts

The number of times the Task has been run.

If the task is currently running, it does not count as an attempt.

</div>

<div class="attribute">

TaskResult.worker_ids

The ids of the workers which have executed the Task.

</div>

</div>

### Task errors

<div class="TaskError">

Contains information about the error raised during the execution of a `Task`.

<div class="attribute">

TaskError.traceback

The traceback (as a string) from the raised exception when the `Task` failed.

</div>

<div class="attribute">

TaskError.exception_class

The exception class raised when executing the `Task`.

</div>

</div>

## Backends

Backends handle how Tasks are stored and executed. All backends share a common interface defined by `BaseTaskBackend`, which specifies the core methods for enqueueing Tasks and retrieving results.

### Base backend

<div class="module">

django.tasks.backends.base

</div>

<div class="BaseTaskBackend">

`BaseTaskBackend` is the parent class for all Task backends.

<div class="attribute">

BaseTaskBackend.task_class

The `~django.tasks.Task` subclass to use when creating tasks with the `~django.tasks.task` decorator. Defaults to `~django.tasks.Task`. Custom backends can override this to use a custom `Task` subclass with additional attributes.

</div>

<div class="attribute">

BaseTaskBackend.options

A dictionary of extra parameters for the Task backend. These are provided using the `OPTIONS <TASKS-OPTIONS>` setting.

</div>

<div class="method">

BaseTaskBackend.enqueue(task, args, kwargs)

Task backends which subclass `BaseTaskBackend` should implement this method as a minimum.

When implemented, `enqueue()` enqueues the `task`, a `.Task` instance, for later execution. `args` are the positional arguments and `kwargs` are the keyword arguments to be passed to the `task`. Returns a `~django.tasks.TaskResult`.

</div>

<div class="method">

BaseTaskBackend.aenqueue(task, args, kwargs)

The `async` variant of `BaseTaskBackend.enqueue`.

</div>

<div class="method">

BaseTaskBackend.get_result(result_id)

Retrieve a result by its id. If the result does not exist, `TaskResultDoesNotExist
<django.tasks.exceptions.TaskResultDoesNotExist>` is raised.

If the backend does not support `get_result()`, `NotImplementedError` is raised.

</div>

<div class="method">

BaseTaskBackend.aget_result(result_id)

The `async` variant of `BaseTaskBackend.get_result`.

</div>

<div class="method">

BaseTaskBackend.validate_task(task)

Validates whether the provided `Task` is able to be enqueued using the backend. If the Task is not valid, `InvalidTask <django.tasks.exceptions.InvalidTask>` is raised.

</div>

</div>

#### Feature flags

Some backends may not support all features Django provides. It's possible to identify the supported functionality of a backend, and potentially change behavior accordingly.

<div class="attribute">

BaseTaskBackend.supports_defer

Whether the backend supports enqueueing Tasks to be executed after a specific time using the `~django.tasks.Task.run_after` attribute.

</div>

<div class="attribute">

BaseTaskBackend.supports_async_task

Whether the backend supports enqueueing async functions (coroutines).

</div>

<div class="attribute">

BaseTaskBackend.supports_get_result

Whether the backend supports retrieving `Task` results from another thread after they have been enqueued.

</div>

<div class="attribute">

BaseTaskBackend.supports_priority

Whether the backend supports executing Tasks as ordered by their `~django.tasks.Task.priority`.

</div>

The below table notes which of the `built-in backends
<task-available-backends>` support which features:

| Feature                | `.DummyBackend` | `.ImmediateBackend` |
|------------------------|-----------------|---------------------|
| `.supports_defer`      | Yes             | No                  |
| `.supports_async_task` | Yes             | Yes                 |
| `.supports_get_result` | No              | No[^1]              |
| `.supports_priority`   | Yes[^2]         | Yes[^3]             |

### Available backends

Django includes only development and testing backends. These support local execution and inspection, for production ready backends refer to `configuring-a-task-backend`.

#### Immediate backend

<div class="module">

django.tasks.backends.immediate

</div>

<div class="ImmediateBackend">

The `immediate backend <immediate-task-backend>` executes Tasks immediately, rather than in the background.

</div>

#### Dummy backend

<div class="module">

django.tasks.backends.dummy

</div>

<div class="DummyBackend">

The `dummy backend <dummy-task-backend>` does not execute enqueued Tasks. Instead, it stores task results for later inspection.

<div class="attribute">

DummyBackend.results

A list of results for the enqueued Tasks, in the order they were enqueued.

</div>

<div class="method">

DummyBackend.clear

Clears the list of stored results.

</div>

</div>

## Exceptions

<div class="module">

django.tasks.exceptions

</div>

<div class="exception">

InvalidTask

Raised when the `.Task` attempting to be enqueued is invalid.

</div>

<div class="exception">

InvalidTaskBackend

Raised when the requested `.BaseTaskBackend` is invalid.

</div>

<div class="exception">

TaskResultDoesNotExist

Raised by `~django.tasks.backends.base.BaseTaskBackend.get_result` when the provided `result_id` does not exist.

</div>

<div class="exception">

TaskResultMismatch

Raised by `~django.tasks.Task.get_result` when the provided `result_id` is for a different Task than the current Task.

</div>

**Footnotes**

[^1]: The `.ImmediateBackend` doesn't officially support `get_result()`, despite implementing the API, since the result cannot be retrieved from a different thread.

[^2]: The `.DummyBackend` has `supports_priority=True` so that it can be used as a drop-in replacement in tests. Since this backend never executes Tasks, the `priority` value has no effect.

[^3]: The `.ImmediateBackend` has `supports_priority=True` so that it can be used as a drop-in replacement in tests. Because Tasks run as soon as they are scheduled, the `priority` value has no effect.
