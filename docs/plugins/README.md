# Plugins known to work with Dory
Please submit PRs with known working plugins to help end-users of Docker Volume plugins get up and running.

<table>
  <tr>
    <th>Docker Volume Plugin</th>
    <th>--driver</th>
    <th>Version</th>
    <th>Status</th>
  </tr>
  <tr>
    <td>HPE Nimble Storage Docker Volume Plugin</td>
    <td>nimble</td>
    <td>2.1.1</td>
    <td>Works</td>
  </tr>
  <tr>
    <td colspan="4"><b>Notes:</b> Tested with K8s 1.5, 1.6, 1.7 and 1.8. OpenShift 3.5 and 3.6. Only tested with the "fat" version included in the Nimble Linux Toolkit available on <a href="https://infosight.nimblestorage.com">InfoSight</a>. The Docker Store version should work as well.</td>
  </tr>
  <tr>
    <td>HPE 3PAR Volume Plug-in for Docker</td>
    <td>hpe</td>
    <td>1.0.0</td>
    <td>Works</td>
  </tr>
  <tr>
    <td colspan="4"><b>Notes:</b> Tested with OpenShift 3.5.</td>
  </tr>
</table>
