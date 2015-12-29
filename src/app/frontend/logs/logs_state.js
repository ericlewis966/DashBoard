// Copyright 2015 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import {LogsController} from './logs_controller';
import LogsToolbarController from './logstoolbar/logstoolbar_controller';
import {toolbarViewName} from '../chrome/chrome_state';

/** Name of the state. Can be used in, e.g., $state.go method. */
export const stateName = 'logs';

/**
 * Parameters for this state.
 *
 * All properties are @exported and in sync with URL param names.
 * @final
 */
export class StateParams {
  /**
   * @param {string} namespace
   * @param {string} replicaSet
   * @param {string} podId
   * @param {string} container
   */
  constructor(namespace, replicaSet, podId, container) {
    /** @export {string} Namespace of this Replica Set. */
    this.namespace = namespace;

    /** @export {string} Name of this Replica Set. */
    this.replicaSet = replicaSet;

    /** @export {string} Id of this Pod. */
    this.podId = podId;

    /** @export {string} Name of this pod container. */
    this.container = container;
  }
}

/**
 * Configures states for the logs view.
 *
 * @param {!ui.router.$stateProvider} $stateProvider
 * @ngInject
 */
export default function stateConfig($stateProvider) {
  let views = {
    '': {
      templateUrl: 'logs/logs.html',
      controller: LogsController,
      controllerAs: 'ctrl',
    },
    [toolbarViewName]: {
      templateUrl: 'logs/logstoolbar/logstoolbar.html',
      controller: LogsToolbarController,
      controllerAs: 'ctrl',
    },
  };

  $stateProvider.state(stateName, {
    url: '/logs/:namespace/:replicaSet/:podId/:container',
    resolve: {
      'replicaSetPods': resolveReplicaSetPods,
      'podLogs': resolvePodLogs,
    },
    views: views,
  });
}

/**
 * @param {!StateParams} $stateParams
 * @param {!angular.$resource} $resource
 * @return {!angular.$q.Promise}
 * @ngInject
 */
function resolveReplicaSetPods($stateParams, $resource) {
  /** @type {!angular.Resource<!backendApi.ReplicaSetPods>} */
  let resource = $resource('/api/replicasets/pods/:namespace/:replicaSet', $stateParams);

  return resource.get().$promise;
}

/**
 * @param {!StateParams} $stateParams
 * @param {!angular.$resource} $resource
 * @return {!angular.$q.Promise}
 * @ngInject
 */
function resolvePodLogs($stateParams, $resource) {
  /** @type {!angular.Resource<!backendApi.Logs>} */
  let resource = $resource('/api/logs/:namespace/:podId/:container', $stateParams);

  return resource.get().$promise;
}
