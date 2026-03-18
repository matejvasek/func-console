## Introduction

This document is meant as a getting started guide for launching a new OpenShift Console Dynamic Plugin UI.  It will guide developers through the processes involved including:  design, planning, and technical implementation

### Why Might I Build a Dynamic Plugin?

An OpenShift Dynamic Plugin provides a mechanism for an OpenShift Operator to deliver a new UI experience to OpenShift users that is decoupled from the OpenShift release cycle.  This allows for quicker updates to be released to the user and reduces technical complexity by removing the context of the broader OpenShift Console codebase.

To ensure compatibility with Console and other plugins, each plugin must declare its dependencies using [semantic version](https://semver.org/) ranges.

## Expectations of Dynamic Plugin Owner Team and Console Engineer Team

The Dynamic Plugin Owner Team is responsible for:

* Working with UXD to draft UX designs for the plugin
* Owning the software lifecycle of the plugin, including implementation, feature development, and maintenance of the plugin
* Understanding customer needs related to plugin features and function

The Console Engineering Team is responsible for:

* Delivering and managing the Dynamic Plugin SDK
  * The Console Engineer team is the primary owner of the SDK, but we welcome contributions per [OpenShift Console Plugin SDK Process](https://docs.google.com/document/d/1v-Ueouq6oHqiv6wk5CwM-babcQ8KnBvaLOMY0S2-RXI/edit?tab=t.0#heading=h.p6ob7nmklfw1)
* Providing consultation to Dynamic Plugin Owner teams as needed
* Providing guidance on best practices for using Dynamic Plugin SDK

## Steps to Launch Dynamic Plugin

Before beginning significant coding work, follow these steps!

1. Document and communicate plugin concept
   1. Create a Jira card with documentation describing the scope and functionality of the Dynamic Plugin
      1. Include mockup designs ([Example](https://docs.google.com/document/d/1acQPWCFjlPTLgxiU9XhOH_r9z2XlOTh5mFdPsdoCqMA/edit?tab=t.0#heading=h.n47p59q9ub8a))
         1. Reach out to your Product Manager if you need help identifying your team's UXD contact to help with designs
   2. Review with Console Team
      1. [Sam Padgett](mailto:spadgett@redhat.com) will review and help with Dynamic Plugin questions + requests
      2. [Ali Mobrem](mailto:amobrem@redhat.com) will review Preliminary Design + optionally bring in a UXD reviewer([Kevin Hatchoua](mailto:khatchou@redhat.com)) for new or complex patterns
      3. For fastest reviews: Just ping us in [#forum-ocp-console](https://redhat.enterprise.slack.com/archives/C6A3NV5J9)!
2. Add plugin information to the [OpenShift Dynamic Plugin Inventory](https://docs.google.com/spreadsheets/d/1wcCdc1s4ewzxtUJ42VdRhAJ9wFA8UwoTajGSftrr5fM/edit?gid=0#gid=0)
   1. Establish a process to ensure this data is kept up to date over time
3. Review Technical Documentation guidance
4. Begin development of your technical implementation and coding efforts.

## Technical Documentation

Refer to the following documentation for technical details around using the Dynamic Plugin SDK and building the plugin.

* [Console Dynamic Plugin SDK documentation](https://github.com/openshift/console/blob/main/frontend/packages/console-dynamic-plugin-sdk/README.md)
  * [Dynamic Plugin template code](https://github.com/openshift/console-plugin-template)
    * Provides minimal template code to launching a new Dynamic Plugin project
  * [Dynamic Plugin Demo repository](https://github.com/openshift/console/tree/main/dynamic-demo-plugin)
    * Provides a demo of how a standalone Dynamic Plugin repository would be structured
* [OpenShift Dynamic Plugin documentation](https://docs.redhat.com/en/documentation/openshift_container_platform/4.20/html/web_console/dynamic-plugins)
* [PatternFly design system documentation](http://patternfly.org)

If a team is interested in contributing to the Dynamic Plugin SDK directly, refer to [OpenShift Console Plugin SDK Process](https://docs.google.com/document/d/1v-Ueouq6oHqiv6wk5CwM-babcQ8KnBvaLOMY0S2-RXI/edit?tab=t.0#heading=h.p6ob7nmklfw1)

### Reporting Bugs

If you encounter a bug with the Dynamic Plugin SDK, please submit an [OpenShift Bug](https://issues.redhat.com/projects/OCPBUGS/issues/) with component "Management Console"

## Communication Forums

### Weekly Dynamic Plugin Sync Forum

[Dynamic Plugins Sync](https://docs.google.com/document/d/19RNON0oY73nJRk_DrI_01IYP_JdmtBt9VkVuOCxTuNs/edit?tab=t.0#heading=h.f2gliic7znqb) occurs on Monday @ 9am ET.

This forum provides an opportunity to bring questions & topics to the Console team and Dynamic Plugin community. It can be used for asking questions, demoing new development, providing feedback on the SDK, process, etc.

### Slack

#### Discussion Forum

The [#forum-ui-extensibility](https://redhat.enterprise.slack.com/archives/C011BL0FEKZ) Slack channel is an open forum where the Console Team and Dynamic Plugin owners can discuss extensions to the OCP Console (Dynamic Plugin implementation, UX considerations, etc.)

#### Announcement Forum

The [#announce-console-plugins](https://redhat.enterprise.slack.com/archives/C032NLNEE8G) Slack channel is used to share announcements related to Dynamic Plugins and the SDK. These include upcoming changes, feature announcements, deprecation notices, etc.
